package support

import (
	"context"
	"runtime"
	"sort"
	"time"

	"github.com/mewisme/mew/internal/app"
)

// SchemaVersion is the canonical support-bundle schema version.
// Increment on breaking manifest or entry shape changes.
const SchemaVersion = 1

// RedactPolicyVersion indicates the redaction policy revision.
const RedactPolicyVersion = "1"

// CollectorVersion is the collector implementation version (this build).
var CollectorVersion = "dev"

// Status values for entries and the overall bundle.
const (
	StatusOK      = "ok"
	StatusError   = "error"
	StatusSkipped = "skipped"
)

// Manifest describes the bundle contents and collection outcome.
type Manifest struct {
	SchemaVersion    int         `json:"schema_version"`
	CollectorVersion string      `json:"collector_version"`
	RedactPolicy     string      `json:"redact_policy"`
	CollectedAt      string      `json:"collected_at"`
	OS               string      `json:"os"`
	Arch             string      `json:"arch"`
	Entries          []EntryMeta `json:"entries"`
	Status           string      `json:"status"`
}

// EntryMeta describes one collected entry in the manifest.
type EntryMeta struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Entry is a collected artifact ready for archive serialization.
type Entry struct {
	Name string
	Data []byte
}

// Collector produces one support-bundle entry from live state.
// Returned data must already be support-safe: redacted, bounded,
// and free of secrets, source contents, env values, and private paths.
type Collector interface {
	// Name returns the stable entry file name (e.g. "version.json").
	Name() string
	// Required reports whether this collector failure fails the whole bundle.
	Required() bool
	// Collect gathers sanitized diagnostic data. A non-nil error marks the
	// entry as errored; for required collectors this fails the bundle.
	Collect(ctx context.Context, ac *app.Context) ([]byte, error)
}

// Collect runs every registered collector and assembles the bundle.
// Required collector failures fail the whole operation. Optional failures
// are recorded as errored entries but do not prevent bundle creation.
func Collect(ctx context.Context, ac *app.Context, collectors []Collector) (*Bundle, error) {
	var entries []Entry
	var metas []EntryMeta
	var firstRequiredErr error

	for _, c := range collectors {
		data, err := c.Collect(ctx, ac)
		meta := EntryMeta{Name: c.Name(), Status: StatusOK}
		if err != nil {
			meta.Status = StatusError
			meta.Error = SanitizeError(err)
			if c.Required() && firstRequiredErr == nil {
				firstRequiredErr = err
			}
		}
		metas = append(metas, meta)
		if data != nil {
			entries = append(entries, Entry{Name: c.Name(), Data: data})
		}
	}

	if firstRequiredErr != nil {
		return nil, firstRequiredErr
	}

	// Sort entries and metas deterministically by name.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	sort.Slice(metas, func(i, j int) bool { return metas[i].Name < metas[j].Name })

	m := Manifest{
		SchemaVersion:    SchemaVersion,
		CollectorVersion: CollectorVersion,
		RedactPolicy:     RedactPolicyVersion,
		CollectedAt:      time.Now().UTC().Format(time.RFC3339),
		OS:               runtime.GOOS,
		Arch:             runtime.GOARCH,
		Entries:          metas,
		Status:           StatusOK,
	}

	return &Bundle{Manifest: m, Entries: entries}, nil
}

// Bundle is a complete support artifact ready for archive serialization.
type Bundle struct {
	Manifest Manifest
	Entries  []Entry
}
