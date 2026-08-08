package support

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/features"
	"github.com/mewisme/mew/internal/node"
)

// DTOs: explicit support-safe types. Never serialize raw production structs.

// VersionDTO is the support-safe version entry.
type VersionDTO struct {
	SchemaVersion int    `json:"schema_version"`
	Version       string `json:"version"`
	Commit        string `json:"commit,omitempty"`
	BuildDate     string `json:"build_date,omitempty"`
	GoVersion     string `json:"go_version"`
}

// OSDTO is the support-safe OS entry.
type OSDTO struct {
	SchemaVersion int    `json:"schema_version"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	NumCPU        int    `json:"num_cpu"`
}

// NodeDTO is the support-safe Node entry.
type NodeDTO struct {
	SchemaVersion int      `json:"schema_version"`
	Version       string   `json:"version,omitempty"`
	Path          string   `json:"path,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
	Source        string   `json:"source,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// FeaturesDTO is the support-safe features summary entry.
type FeaturesDTO struct {
	SchemaVersion    int            `json:"schema_version"`
	SchemaVersionRef string         `json:"schema_version_ref,omitempty"`
	Total            int            `json:"total"`
	ByStatus         map[string]int `json:"by_status"`
}

// DoctorDTO is the support-safe doctor entry (reuses app.DoctorReport shape).
type DoctorDTO struct {
	SchemaVersion int              `json:"schema_version"`
	CheckedAt     string           `json:"checked_at,omitempty"`
	Checks        []DoctorCheckDTO `json:"checks"`
	OK            bool             `json:"ok"`
	Error         string           `json:"error,omitempty"`
}

// DoctorCheckDTO is a support-safe doctor check.
type DoctorCheckDTO struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	Details     string `json:"details,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

// ConfigMetaDTO is the support-safe config metadata entry.
type ConfigMetaDTO struct {
	SchemaVersion int      `json:"schema_version"`
	Keys          []string `json:"keys,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// Collectors ------------------------------------------------------------------

// VersionCollector gathers Mew version and build information.
type VersionCollector struct{}

func (VersionCollector) Name() string   { return "version.json" }
func (VersionCollector) Required() bool { return true }

func (VersionCollector) Collect(ctx context.Context, ac *app.Context) ([]byte, error) {
	dto := VersionDTO{
		SchemaVersion: 1,
		GoVersion:     strings.TrimPrefix(runtime.Version(), "go"),
	}
	if ac != nil {
		dto.Version = ac.Version
		dto.Commit = ac.Commit
		dto.BuildDate = ac.BuildDate
	}
	return json.Marshal(dto)
}

// OSCollector gathers host OS and architecture information.
type OSCollector struct{}

func (OSCollector) Name() string   { return "os.json" }
func (OSCollector) Required() bool { return true }

func (OSCollector) Collect(ctx context.Context, ac *app.Context) ([]byte, error) {
	dto := OSDTO{
		SchemaVersion: 1,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		NumCPU:        runtime.NumCPU(),
	}
	return json.Marshal(dto)
}

// NodeCollector gathers Node version and capability information.
type NodeCollector struct{}

func (NodeCollector) Name() string   { return "node.json" }
func (NodeCollector) Required() bool { return false }

func (NodeCollector) Collect(ctx context.Context, ac *app.Context) ([]byte, error) {
	dto := NodeDTO{SchemaVersion: 1}
	inst, err := node.Discover(ctx, node.Request{})
	if err != nil {
		dto.Error = SanitizeError(err)
		return json.Marshal(dto)
	}
	dto.Version = inst.NormalizedVersion
	dto.Path = SafePath(inst.ExePath, cwdFrom(ac))
	dto.Capabilities = inst.Capabilities
	dto.Source = inst.DiscoverySource
	return json.Marshal(dto)
}

// FeaturesCollector gathers the feature inventory summary.
type FeaturesCollector struct{}

func (FeaturesCollector) Name() string   { return "features.json" }
func (FeaturesCollector) Required() bool { return false }

func (FeaturesCollector) Collect(ctx context.Context, ac *app.Context) ([]byte, error) {
	dto := FeaturesDTO{SchemaVersion: 1, ByStatus: map[string]int{}}
	inv, err := features.LoadEmbedded()
	if err != nil {
		// Not fatal; features inventory is embedded and should always load.
		// Return the DTO with empty data rather than failing entirely.
		return json.Marshal(dto)
	}
	dto.SchemaVersionRef = inv.SchemaVersion
	dto.Total = len(inv.Features)
	for _, f := range inv.Features {
		dto.ByStatus[string(f.MewStatus)]++
	}
	return json.Marshal(dto)
}

// DoctorCollector gathers the runtime doctor report.
type DoctorCollector struct{}

func (DoctorCollector) Name() string   { return "doctor.json" }
func (DoctorCollector) Required() bool { return false }

func (DoctorCollector) Collect(ctx context.Context, ac *app.Context) ([]byte, error) {
	dto := DoctorDTO{SchemaVersion: 1}
	if ac == nil {
		dto.Error = "no app context"
		return json.Marshal(dto)
	}
	rep, err := app.DoctorRuntime(ctx, ac, app.DoctorOptions{})
	if err != nil {
		dto.Error = SanitizeError(err)
		return json.Marshal(dto)
	}
	dto.CheckedAt = rep.CheckedAt
	dto.OK = rep.OK
	for _, c := range rep.Checks {
		dto.Checks = append(dto.Checks, DoctorCheckDTO{
			ID:          c.ID,
			Status:      c.Status,
			Message:     SanitizeString(c.Message),
			Details:     SanitizeString(c.Details),
			Remediation: c.Remediation,
		})
	}
	return json.Marshal(dto)
}

// ConfigMetaCollector gathers sanitized config metadata.
type ConfigMetaCollector struct{}

func (ConfigMetaCollector) Name() string   { return "config.json" }
func (ConfigMetaCollector) Required() bool { return false }

func (ConfigMetaCollector) Collect(ctx context.Context, ac *app.Context) ([]byte, error) {
	dto := ConfigMetaDTO{SchemaVersion: 1}
	if ac == nil || ac.Config == nil {
		dto.Error = "no config loaded"
		return json.Marshal(dto)
	}
	for k, v := range ac.Config.Values {
		// Only include non-secret key names, never values.
		if SanitizeString(k) != k {
			continue
		}
		_ = v // never serialize config values
		dto.Keys = append(dto.Keys, k)
	}
	return json.Marshal(dto)
}

// cwdFrom returns the CWD from the app context, or empty string.
func cwdFrom(ac *app.Context) string {
	if ac == nil {
		return ""
	}
	return ac.CWD
}
