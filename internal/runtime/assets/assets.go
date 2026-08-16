// Package assets holds embedded Node loader and preload sources.
package assets

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mewisme/mew/internal/apperr"
)

//go:embed preload.cjs
var PreloadCJS []byte

//go:embed preload.mjs
var PreloadMJS []byte

//go:embed manifest.json
var manifestRaw []byte

//go:embed preload.cjs preload.mjs loader-register.mjs ts-loader.mjs credential-grabber.cjs web-storage.cjs manifest.json resolve-utils.mjs resolve-diagnostic.mjs
var runtimeFS embed.FS

// AssetRole classifies how an asset is injected into Node.
type AssetRole string

const (
	RolePreloadCJS         AssetRole = "preload-cjs"
	RolePreloadESM         AssetRole = "preload-esm"
	RoleLoaderRegistration AssetRole = "loader-registration"
	RoleLoaderSupport      AssetRole = "loader-support"
	RoleCredentialGrabber  AssetRole = "credential-grabber"
)

// Injected reports whether the asset is injected into Node argv.
func (r AssetRole) Injected() bool {
	switch r {
	case RolePreloadCJS, RolePreloadESM, RoleLoaderRegistration, RoleCredentialGrabber:
		return true
	default:
		return false
	}
}

// ManifestEntry is a single asset entry in the runtime manifest.
type ManifestEntry struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Role       AssetRole `json:"role,omitempty"`
	ModuleType string    `json:"moduleType"`
	Size       int64     `json:"size"`
	SHA256     string    `json:"sha256"`
}

// AssetManifest lists all embedded runtime assets.
type AssetManifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	BundleVersion string          `json:"bundleVersion"`
	Assets        []ManifestEntry `json:"assets"`
}

// LoadManifest reads, validates, and normalizes the embedded manifest.
func LoadManifest() (*AssetManifest, error) {
	var m AssetManifest
	if err := json.Unmarshal(manifestRaw, &m); err != nil {
		return nil, apperr.Wrap(apperr.RuntimeAssetManifest, "assets.manifest", "", err)
	}
	if err := ValidateManifest(&m); err != nil {
		return nil, err
	}
	normalizeManifestRoles(&m)
	if err := validateManifestRoles(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// validateManifestRoles checks all roles are known after normalization.
func validateManifestRoles(m *AssetManifest) error {
	for i := range m.Assets {
		e := &m.Assets[i]
		switch e.Role {
		case RolePreloadCJS, RolePreloadESM, RoleLoaderRegistration, RoleLoaderSupport, RoleCredentialGrabber:
			// valid
		default:
			return apperr.New(apperr.RuntimeAssetManifest, "assets.manifest", e.Name,
				fmt.Sprintf("unknown asset role %q", e.Role))
		}
	}
	return nil
}

// ValidateManifest checks the manifest for structural and security invariants.
func ValidateManifest(m *AssetManifest) error {
	if m.SchemaVersion != 1 && m.SchemaVersion != 2 {
		return apperr.New(apperr.RuntimeAssetManifest, "assets.manifest", "",
			fmt.Sprintf("unsupported manifest schema version %d", m.SchemaVersion))
	}
	if m.BundleVersion == "" {
		return apperr.New(apperr.RuntimeAssetManifest, "assets.manifest", "", "missing bundle version")
	}

	seenNames := map[string]bool{}
	seenPaths := map[string]bool{}
	sha256RE := regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

	for i := range m.Assets {
		e := &m.Assets[i]

		// Unique logical name.
		if seenNames[e.Name] {
			return apperr.New(apperr.RuntimeAssetManifest, "assets.manifest", e.Name,
				"duplicate asset name")
		}
		seenNames[e.Name] = true

		// Unique extraction path.
		if seenPaths[e.Path] {
			return apperr.New(apperr.RuntimeAssetManifest, "assets.manifest", e.Path,
				"duplicate asset path")
		}
		seenPaths[e.Path] = true

		// No path traversal or absolute paths.
		clean := filepath.Clean(e.Path)
		if strings.Contains(clean, "..") || filepath.IsAbs(clean) {
			return apperr.New(apperr.RuntimeAssetManifest, "assets.manifest", e.Path,
				"asset path must be relative and must not contain ..")
		}

		// SHA-256 format: 64 hex chars.
		if !sha256RE.MatchString(e.SHA256) {
			return apperr.New(apperr.RuntimeAssetManifest, "assets.manifest", e.Name,
				fmt.Sprintf("invalid SHA-256 digest %q", e.SHA256))
		}

		// Declared size must match embedded source.
		data, err := ReadAsset(e.Path)
		if err != nil {
			return apperr.Wrap(apperr.RuntimeAssetManifest, "assets.manifest", e.Path, err)
		}
		if int64(len(data)) != e.Size {
			return apperr.New(apperr.RuntimeAssetManifest, "assets.manifest", e.Name,
				fmt.Sprintf("declared size %d does not match embedded size %d", e.Size, len(data)))
		}

		// Module type must be known.
		switch e.ModuleType {
		case "cjs", "esm":
		default:
			return apperr.New(apperr.RuntimeAssetManifest, "assets.manifest", e.Name,
				fmt.Sprintf("unknown module type %q", e.ModuleType))
		}
	}
	return nil
}

// normalizeManifestRoles backfills Role from ModuleType for schema v1 manifests
// and validates v2 roles.
func normalizeManifestRoles(m *AssetManifest) {
	for i := range m.Assets {
		e := &m.Assets[i]
		if e.Role == "" {
			// Schema v1: derive Role from ModuleType.
			switch e.ModuleType {
			case "cjs":
				e.Role = RolePreloadCJS
			case "esm":
				e.Role = RolePreloadESM
			default:
				e.Role = RoleLoaderSupport
			}
		}
	}
}

// AssetsSorted returns the sorted list of manifest entries.
func (m *AssetManifest) AssetsSorted() []ManifestEntry {
	sorted := make([]ManifestEntry, len(m.Assets))
	copy(sorted, m.Assets)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}

// ReadAsset reads an embedded asset by name.
func ReadAsset(name string) ([]byte, error) {
	data, err := fs.ReadFile(runtimeFS, name)
	if err != nil {
		return nil, apperr.Wrap(apperr.RuntimeAssetCache, "assets.read", name, err)
	}
	return data, nil
}

// VerifyAsset checks an extracted asset against the expected SHA-256 digest.
func VerifyAsset(r io.Reader, expectedSHA256 string) error {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return apperr.Wrap(apperr.RuntimeAssetDigest, "assets.verify", "", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, expectedSHA256) {
		return apperr.New(apperr.RuntimeAssetDigest, "assets.verify", "",
			fmt.Sprintf("digest mismatch: expected %s, got %s", expectedSHA256, got))
	}
	return nil
}
