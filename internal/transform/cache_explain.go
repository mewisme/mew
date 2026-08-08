package transform

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CacheDisposition is the final disposition of a cache entry.
type CacheDisposition string

const (
	CacheDispositionHit         CacheDisposition = "hit"
	CacheDispositionMiss        CacheDisposition = "miss"
	CacheDispositionCorrupt     CacheDisposition = "corrupt"
	CacheDispositionSchemaStale CacheDisposition = "schema-stale"
	CacheDispositionOrphan      CacheDisposition = "orphan"
	CacheDispositionUnreadable  CacheDisposition = "unreadable"
)

// CacheReasonCode is a stable machine-readable reason code.
type CacheReasonCode string

const (
	ReasonHit             CacheReasonCode = "hit"
	ReasonMissEntryAbsent CacheReasonCode = "miss-entry-absent"
	ReasonKeyMismatch     CacheReasonCode = "key-mismatch"
	ReasonSchemaMismatch  CacheReasonCode = "schema-mismatch"
	ReasonSourceMismatch  CacheReasonCode = "source-mismatch"
	ReasonOptionsMismatch CacheReasonCode = "options-mismatch"
	ReasonConfigMismatch  CacheReasonCode = "config-mismatch"
	ReasonFormatMismatch  CacheReasonCode = "format-mismatch"
	ReasonMapModeMismatch CacheReasonCode = "map-mode-mismatch"
	ReasonMetaMalformed   CacheReasonCode = "meta-malformed"
	ReasonCodeMissing     CacheReasonCode = "code-missing"
	ReasonMapMissing      CacheReasonCode = "map-missing"
	ReasonDigestMismatch  CacheReasonCode = "digest-mismatch"
	ReasonOutputMismatch  CacheReasonCode = "output-mismatch"
	ReasonOrphanMeta      CacheReasonCode = "orphan-meta"
	ReasonOrphanFile      CacheReasonCode = "orphan-file"
	ReasonUnreadableMeta  CacheReasonCode = "unreadable-meta"
	ReasonUnreadableCode  CacheReasonCode = "unreadable-code"
	ReasonUnreadableMap   CacheReasonCode = "unreadable-map"
	ReasonStatError       CacheReasonCode = "stat-error"
	ReasonReadError       CacheReasonCode = "read-error"
)

// CacheReason is a single structured explanation of a cache disposition.
type CacheReason struct {
	Code     CacheReasonCode `json:"code"`
	Severity string          `json:"severity"` // info, warn, error
	Message  string          `json:"message"`
}

// CacheEntryExplain describes the disposition of a single cache entry.
type CacheEntryExplain struct {
	Key         string           `json:"key"`
	Disposition CacheDisposition `json:"disposition"`
	Reasons     []CacheReason    `json:"reasons,omitempty"`
	SchemaVer   int              `json:"schemaVersion,omitempty"`
	CodeSize    int64            `json:"codeBytes,omitempty"`
	MapSize     int64            `json:"mapBytes,omitempty"`
	MetaSize    int64            `json:"metaBytes,omitempty"`
}

// CacheExplainOptions configures the cache explain diagnostic.
type CacheExplainOptions struct {
	// Key is an optional specific cache key to explain.
	Key string
}

// CacheExplainResult is the authoritative cache diagnostic result.
type CacheExplainResult struct {
	CacheDir   string             `json:"cacheDir"`
	SchemaVer  int                `json:"schemaVersion"`
	EntryCount int                `json:"entryCount"`
	CodeBytes  int64              `json:"codeBytes"`
	MapBytes   int64              `json:"mapBytes"`
	MetaBytes  int64              `json:"metaBytes"`
	TotalBytes int64              `json:"totalBytes"`
	Entry      *CacheEntryExplain `json:"entry,omitempty"`
	Orphans    []CacheReason      `json:"orphans,omitempty"`
	Errors     []CacheReason      `json:"errors,omitempty"`
}

// CacheExplain runs the authoritative cache diagnostic using the production
// cache key/schema/digest/read path.
func CacheExplain(cacheDir string, opts CacheExplainOptions) (*CacheExplainResult, error) {
	result := &CacheExplainResult{
		CacheDir:  cacheDir,
		SchemaVer: CacheSchemaVersion,
	}

	// Scan the cache directory.
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("reading cache dir %s: %w", cacheDir, err)
	}

	// Build a map of expected artifacts for orphan detection.
	expected := make(map[string]bool) // full path -> true if accounted for by a .meta entry

	// Walk prefix dirs to count entries and collect metadata.
	for _, prefixEntry := range entries {
		if !prefixEntry.IsDir() || len(prefixEntry.Name()) != 2 {
			continue
		}
		prefixDir := filepath.Join(cacheDir, prefixEntry.Name())
		keyEntries, err := os.ReadDir(prefixDir)
		if err != nil {
			result.Errors = append(result.Errors, CacheReason{
				Code:     ReasonStatError,
				Severity: "error",
				Message:  fmt.Sprintf("cannot read prefix dir %s: %v", prefixEntry.Name(), err),
			})
			continue
		}
		for _, keyEntry := range keyEntries {
			if keyEntry.IsDir() {
				continue
			}
			fullPath := filepath.Join(prefixDir, keyEntry.Name())
			ext := filepath.Ext(keyEntry.Name())

			if ext == ".meta" {
				baseName := keyEntry.Name()[:len(keyEntry.Name())-5]
				metaPath := fullPath
				codePath := filepath.Join(prefixDir, baseName+".code")
				mapPath := filepath.Join(prefixDir, baseName+".map")

				expected[metaPath] = true
				expected[codePath] = true
				expected[mapPath] = true

				// Account for file sizes.
				if st, err := os.Stat(metaPath); err == nil {
					result.MetaBytes += st.Size()
					result.EntryCount++
				}
				if st, err := os.Stat(codePath); err == nil {
					result.CodeBytes += st.Size()
				}
				if st, err := os.Stat(mapPath); err == nil {
					result.MapBytes += st.Size()
				}
			}
		}
	}

	// Detect orphan files.
	for _, prefixEntry := range entries {
		if !prefixEntry.IsDir() || len(prefixEntry.Name()) != 2 {
			continue
		}
		prefixDir := filepath.Join(cacheDir, prefixEntry.Name())
		keyEntries, err := os.ReadDir(prefixDir)
		if err != nil {
			continue
		}
		for _, keyEntry := range keyEntries {
			if keyEntry.IsDir() {
				continue
			}
			fullPath := filepath.Join(prefixDir, keyEntry.Name())
			if !expected[fullPath] {
				ext := filepath.Ext(keyEntry.Name())
				code := ReasonOrphanFile
				if ext == ".meta" {
					code = ReasonOrphanMeta
				}
				result.Orphans = append(result.Orphans, CacheReason{
					Code:     code,
					Severity: "warn",
					Message:  fmt.Sprintf("orphan file: %s", filepath.Join(prefixEntry.Name(), keyEntry.Name())),
				})
			}
		}
	}

	result.TotalBytes = result.CodeBytes + result.MapBytes + result.MetaBytes

	// Sort orphans and errors deterministically.
	sort.Slice(result.Orphans, func(i, j int) bool {
		return result.Orphans[i].Message < result.Orphans[j].Message
	})
	sort.Slice(result.Errors, func(i, j int) bool {
		return result.Errors[i].Message < result.Errors[j].Message
	})

	// Explain a specific key if requested.
	if opts.Key != "" {
		result.Entry = explainCacheEntry(cacheDir, opts.Key)
	}

	return result, nil
}

// explainCacheEntry diagnoses a single cache entry using the production
// TryReadCache path plus metadata inspection.
func explainCacheEntry(cacheDir, key string) *CacheEntryExplain {
	entry := &CacheEntryExplain{
		Key: key,
	}

	entryPath := CacheKeyPath(cacheDir, key)
	metaPath := entryPath + ".meta"
	codePath := entryPath + ".code"
	mapPath := entryPath + ".map"

	// Check if the entry exists at all.
	metaData, metaErr := os.ReadFile(metaPath)
	if metaErr != nil {
		if os.IsNotExist(metaErr) {
			entry.Disposition = CacheDispositionMiss
			entry.Reasons = append(entry.Reasons, CacheReason{
				Code:     ReasonMissEntryAbsent,
				Severity: "info",
				Message:  "no cache entry for this key",
			})
			return entry
		}
		entry.Disposition = CacheDispositionUnreadable
		entry.Reasons = append(entry.Reasons, CacheReason{
			Code:     ReasonUnreadableMeta,
			Severity: "error",
			Message:  fmt.Sprintf("cannot read metadata: %v", metaErr),
		})
		return entry
	}

	// Account for sizes.
	if st, err := os.Stat(metaPath); err == nil {
		entry.MetaSize = st.Size()
	}
	if st, err := os.Stat(codePath); err == nil {
		entry.CodeSize = st.Size()
	}
	if st, err := os.Stat(mapPath); err == nil {
		entry.MapSize = st.Size()
	}

	// Parse metadata.
	var e cacheEntry
	if err := json.Unmarshal(metaData, &e); err != nil {
		entry.Disposition = CacheDispositionCorrupt
		entry.Reasons = append(entry.Reasons, CacheReason{
			Code:     ReasonMetaMalformed,
			Severity: "error",
			Message:  fmt.Sprintf("metadata malformed: %v", err),
		})
		return entry
	}
	entry.SchemaVer = e.SchemaVersion

	// Schema version check.
	if e.SchemaVersion != CacheSchemaVersion {
		entry.Disposition = CacheDispositionSchemaStale
		entry.Reasons = append(entry.Reasons, CacheReason{
			Code:     ReasonSchemaMismatch,
			Severity: "warn",
			Message:  fmt.Sprintf("schema version %d != current %d", e.SchemaVersion, CacheSchemaVersion),
		})
		return entry
	}

	// Check code file.
	codeData, codeErr := os.ReadFile(codePath)
	if codeErr != nil {
		if os.IsNotExist(codeErr) {
			entry.Disposition = CacheDispositionCorrupt
			entry.Reasons = append(entry.Reasons, CacheReason{
				Code:     ReasonCodeMissing,
				Severity: "error",
				Message:  "code artifact missing",
			})
		} else {
			entry.Disposition = CacheDispositionUnreadable
			entry.Reasons = append(entry.Reasons, CacheReason{
				Code:     ReasonUnreadableCode,
				Severity: "error",
				Message:  fmt.Sprintf("cannot read code: %v", codeErr),
			})
		}
		return entry
	}

	// Code digest check.
	actualCodeDigest := digestBytes(codeData)
	if actualCodeDigest != e.CodeDigest {
		entry.Disposition = CacheDispositionCorrupt
		entry.Reasons = append(entry.Reasons, CacheReason{
			Code:     ReasonDigestMismatch,
			Severity: "error",
			Message:  fmt.Sprintf("code digest mismatch: expected %s, got %s", e.CodeDigest, actualCodeDigest),
		})
		return entry
	}

	// Check map if metadata references one.
	if e.MapDigest != "" {
		mapData, mapErr := os.ReadFile(mapPath)
		if mapErr != nil {
			if os.IsNotExist(mapErr) {
				entry.Disposition = CacheDispositionCorrupt
				entry.Reasons = append(entry.Reasons, CacheReason{
					Code:     ReasonMapMissing,
					Severity: "error",
					Message:  "map artifact missing",
				})
			} else {
				entry.Disposition = CacheDispositionUnreadable
				entry.Reasons = append(entry.Reasons, CacheReason{
					Code:     ReasonUnreadableMap,
					Severity: "error",
					Message:  fmt.Sprintf("cannot read map: %v", mapErr),
				})
			}
			return entry
		}

		actualMapDigest := digestBytes(mapData)
		if actualMapDigest != e.MapDigest {
			entry.Disposition = CacheDispositionCorrupt
			entry.Reasons = append(entry.Reasons, CacheReason{
				Code:     ReasonDigestMismatch,
				Severity: "error",
				Message:  fmt.Sprintf("map digest mismatch: expected %s, got %s", e.MapDigest, actualMapDigest),
			})
			return entry
		}
	}

	// Output digest check.
	expectedOutput := computeOutputDigest(codeData, nil)
	if e.MapDigest != "" {
		mapData, _ := os.ReadFile(mapPath)
		expectedOutput = computeOutputDigest(codeData, mapData)
	}
	if e.OutputDigest != expectedOutput {
		entry.Disposition = CacheDispositionCorrupt
		entry.Reasons = append(entry.Reasons, CacheReason{
			Code:     ReasonOutputMismatch,
			Severity: "error",
			Message:  fmt.Sprintf("output digest mismatch: expected %s, got %s", e.OutputDigest, expectedOutput),
		})
		return entry
	}

	// All checks passed: it's a hit.
	// Do a full TryReadCache to confirm the production read path agrees.
	cached, readErr := TryReadCache(cacheDir, key)
	if readErr != nil {
		entry.Disposition = CacheDispositionCorrupt
		entry.Reasons = append(entry.Reasons, CacheReason{
			Code:     ReasonReadError,
			Severity: "error",
			Message:  fmt.Sprintf("production read error: %v", readErr),
		})
		return entry
	}
	if cached == nil {
		entry.Disposition = CacheDispositionCorrupt
		entry.Reasons = append(entry.Reasons, CacheReason{
			Code:     ReasonReadError,
			Severity: "error",
			Message:  "production read returned nil despite passing all checks",
		})
		return entry
	}

	entry.Disposition = CacheDispositionHit
	entry.Reasons = append(entry.Reasons, CacheReason{
		Code:     ReasonHit,
		Severity: "info",
		Message:  "cache hit — all integrity checks passed",
	})

	// Add an optional Info-level reason to note what the entry contains.
	extra := make([]string, 0, 2)
	if e.MapDigest != "" {
		extra = append(extra, "source-map")
	}
	if len(extra) > 0 {
		entry.Reasons = append(entry.Reasons, CacheReason{
			Code:     ReasonHit,
			Severity: "info",
			Message:  "contains: " + strings.Join(extra, ", "),
		})
	}

	return entry
}
