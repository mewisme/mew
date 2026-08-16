package support

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mewisme/mew/internal/fsx"
)

// epoch is the deterministic timestamp used for all tar entries.
var epoch = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// manifestEntryName is the manifest file name inside the archive.
const manifestEntryName = "manifest.json"

// WriteBundle archives the bundle as a deterministic gzipped tar to path.
//
// The archive is built entirely in memory then written atomically via
// fsx.WriteAtomic. On failure no partial artifact remains at path.
// Archive permissions are 0o644 (readable by all, no execute).
func WriteBundle(path string, b *Bundle) error {
	data, err := buildArchive(b)
	if err != nil {
		return fmt.Errorf("support: build archive: %w", err)
	}
	if err := fsx.WriteAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("support: write archive: %w", err)
	}
	return nil
}

// buildArchive creates a deterministic gzipped tar in memory.
func buildArchive(b *Bundle) ([]byte, error) {
	var buf bytes.Buffer

	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Manifest first.
	manifestData, err := json.MarshalIndent(b.Manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')
	if err := writeTarEntry(tw, manifestEntryName, manifestData); err != nil {
		return nil, err
	}

	// Then entries in sorted order (already sorted by Collect).
	for _, e := range b.Entries {
		if err := writeTarEntry(tw, e.Name, e.Data); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("close gzip: %w", err)
	}

	return buf.Bytes(), nil
}

// writeTarEntry adds a single file entry to the tar writer.
func writeTarEntry(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    0o644,
		Size:    int64(len(data)),
		ModTime: epoch,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write header %s: %w", name, err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("write body %s: %w", name, err)
	}
	return nil
}

// WriteBundleToDir writes the bundle entries as individual files to dir
// for debugging and testing. Not used in production; production always uses
// WriteBundle for the atomic archive path.
func WriteBundleToDir(dir string, b *Bundle) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Manifest.
	manifestData, err := json.MarshalIndent(b.Manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')
	if err := fsx.WriteAtomic(filepath.Join(dir, manifestEntryName), manifestData, 0o644); err != nil {
		return err
	}

	// Entries.
	for _, e := range b.Entries {
		if err := fsx.WriteAtomic(filepath.Join(dir, e.Name), e.Data, 0o644); err != nil {
			return err
		}
	}

	return nil
}

// ReadBundle reads a bundle archive from the given path.
func ReadBundle(path string) (*Bundle, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	var manifest Manifest
	var entries []Entry

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar reader: %w", err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", hdr.Name, err)
		}
		if hdr.Name == manifestEntryName {
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, fmt.Errorf("decode manifest: %w", err)
			}
		} else {
			entries = append(entries, Entry{Name: hdr.Name, Data: data})
		}
	}

	return &Bundle{Manifest: manifest, Entries: entries}, nil
}
