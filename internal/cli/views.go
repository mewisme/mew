package cli

import (
	"strconv"

	"github.com/mewisme/mew/internal/app"
	"github.com/mewisme/mew/internal/lifecycle"
	"github.com/mewisme/mew/internal/policy"
	"github.com/mewisme/mew/internal/presentation"
	"github.com/mewisme/mew/internal/snapshot"
)

func doctorTableModel(rep app.DoctorReport) presentation.TableModel {
	cols := []presentation.TableColumn{
		{Key: "check", Header: "CHECK", MinWidth: 8, Prefer: 20, Primary: true},
		{Key: "status", Header: "STATUS", MinWidth: 4, Prefer: 8},
		{Key: "message", Header: "MESSAGE", MinWidth: 8, Prefer: 40, Truncate: presentation.TruncateMiddle},
	}
	rows := make([]map[string]string, 0, len(rep.Checks))
	statuses := make([]map[string]presentation.StatusCell, 0, len(rep.Checks))
	for _, c := range rep.Checks {
		rows = append(rows, map[string]string{
			"check":   c.ID,
			"status":  c.Status,
			"message": c.Message,
		})
		statuses = append(statuses, map[string]presentation.StatusCell{
			"status": {Text: c.Status, Status: doctorStatusToPresentation(c.Status)},
		})
	}
	return presentation.TableModel{Columns: cols, Rows: rows, RowStatuses: statuses}
}

func doctorStatusToPresentation(s string) presentation.Status {
	switch app.DoctorCheckStatus(s) {
	case app.DoctorStatusOK:
		return presentation.StatusSuccess
	case app.DoctorStatusWarn:
		return presentation.StatusWarning
	case app.DoctorStatusFail:
		return presentation.StatusError
	case app.DoctorStatusSkipped:
		return presentation.StatusSkipped
	default:
		return presentation.StatusInfo
	}
}

func doctorSummary(rep app.DoctorReport) presentation.Summary {
	st := presentation.StatusSuccess
	title := "Health checks passed"
	if !rep.OK {
		st = presentation.StatusError
		title = "Health checks failed"
	}
	return presentation.Summary{Status: st, Title: title}
}

func outdatedTableModel(entries []app.OutdatedEntry) presentation.TableModel {
	cols := []presentation.TableColumn{
		{Key: "package", Header: "PACKAGE", MinWidth: 8, Prefer: 24, Primary: true, Truncate: presentation.TruncateMiddle, CellStyle: presentation.ValuePackage},
		{Key: "current", Header: "CURRENT", MinWidth: 6, Prefer: 12, CellStyle: presentation.ValueVersion},
		{Key: "wanted", Header: "WANTED", MinWidth: 6, Prefer: 12, CellStyle: presentation.ValueVersion},
		{Key: "latest", Header: "LATEST", MinWidth: 6, Prefer: 12, CellStyle: presentation.ValueVersion},
		{Key: "location", Header: "LOCATION", MinWidth: 4, Prefer: 16, Truncate: presentation.TruncateMiddle, CellStyle: presentation.ValuePath},
	}
	rows := make([]map[string]string, 0, len(entries))
	for _, e := range entries {
		loc := e.Importer
		if loc == "" || loc == "." {
			loc = "."
		}
		rows = append(rows, map[string]string{
			"package":  e.Package,
			"current":  e.Current,
			"wanted":   e.Wanted,
			"latest":   e.Latest,
			"location": loc,
		})
	}
	return presentation.TableModel{Columns: cols, Rows: rows}
}

func workspaceListTable(rows []workspaceListRow) presentation.TableModel {
	cols := []presentation.TableColumn{
		{Key: "name", Header: "NAME", MinWidth: 8, Prefer: 20, Primary: true, CellStyle: presentation.ValuePackage},
		{Key: "version", Header: "VERSION", MinWidth: 6, Prefer: 12, CellStyle: presentation.ValueVersion},
		{Key: "path", Header: "PATH", MinWidth: 4, Prefer: 24, Truncate: presentation.TruncateMiddle, CellStyle: presentation.ValuePath},
	}
	tableRows := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		tableRows = append(tableRows, map[string]string{
			"name":    r.Name,
			"version": r.Version,
			"path":    r.Path,
		})
	}
	return presentation.TableModel{Columns: cols, Rows: tableRows}
}

type workspaceListRow struct {
	Name    string
	Version string
	Path    string
}

func storeStatusView(st app.StoreStatus) []presentation.KeyValue {
	return []presentation.KeyValue{
		{Key: "path", Value: st.Path, Style: presentation.ValuePath},
		{Key: "packages", Value: strconv.Itoa(st.PackageCount), Style: presentation.ValueNumber},
		{Key: "bytes", Value: strconv.FormatInt(st.Bytes, 10), Style: presentation.ValueNumber},
	}
}

func cacheVerifySummary(ok, bad, skip int) presentation.Summary {
	st := presentation.StatusSuccess
	title := "Cache verification passed"
	if bad > 0 {
		st = presentation.StatusError
		title = "Cache verification failed"
	}
	return presentation.Summary{
		Status: st,
		Title:  title,
		Metrics: []presentation.KeyValue{
			{Key: "ok", Value: strconv.Itoa(ok), Style: presentation.ValueNumber},
			{Key: "bad", Value: strconv.Itoa(bad), Style: presentation.ValueNumber},
			{Key: "skip", Value: strconv.Itoa(skip), Style: presentation.ValueNumber},
		},
	}
}

func snapshotTableModel(list []snapshot.Snapshot, sym presentation.Symbols) presentation.TableModel {
	cols := []presentation.TableColumn{
		{Key: "id", Header: "ID", MinWidth: 8, Prefer: 16, Primary: true, Truncate: presentation.TruncateMiddle},
		{Key: "created", Header: "CREATED", MinWidth: 8, Prefer: 24},
		{Key: "digest", Header: "DIGEST", MinWidth: 8, Prefer: 12, Truncate: presentation.TruncateMiddle},
		{Key: "delta", Header: "DELTA", MinWidth: 4, Prefer: 16},
	}
	rows := make([]map[string]string, 0, len(list))
	for i, s := range list {
		var older *snapshot.Snapshot
		if i+1 < len(list) {
			older = &list[i+1]
		}
		rows = append(rows, map[string]string{
			"id":      s.ID,
			"created": s.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			"digest":  shortDigest(s.GraphDigest, sym),
			"delta":   snapshotDeltaSummary(s, older),
		})
	}
	return presentation.TableModel{Columns: cols, Rows: rows}
}

func policyTableModel(result policy.PolicyResult) presentation.TableModel {
	cols := []presentation.TableColumn{
		{Key: "severity", Header: "SEVERITY", MinWidth: 6, Prefer: 10},
		{Key: "package", Header: "PACKAGE", MinWidth: 8, Prefer: 24, Primary: true, Truncate: presentation.TruncateMiddle, CellStyle: presentation.ValuePackage},
		{Key: "message", Header: "MESSAGE", MinWidth: 8, Prefer: 40, Truncate: presentation.TruncateMiddle},
	}
	rows := make([]map[string]string, 0, len(result.Violations))
	statuses := make([]map[string]presentation.StatusCell, 0, len(result.Violations))
	for _, v := range result.Violations {
		rows = append(rows, map[string]string{
			"severity": string(v.Severity),
			"package":  v.Package,
			"message":  v.Message,
		})
		statuses = append(statuses, map[string]presentation.StatusCell{
			"severity": {Text: string(v.Severity), Status: policySeverityToStatus(v.Severity)},
		})
	}
	return presentation.TableModel{Columns: cols, Rows: rows, RowStatuses: statuses}
}

func policySeverityToStatus(s policy.SeverityLevel) presentation.Status {
	switch s {
	case policy.SeverityError:
		return presentation.StatusError
	case policy.SeverityWarn:
		return presentation.StatusWarning
	default:
		return presentation.StatusInfo
	}
}

func policySummary(result policy.PolicyResult) presentation.Summary {
	if result.Passed && len(result.Violations) == 0 {
		return presentation.Summary{Status: presentation.StatusSuccess, Title: "Policy check passed"}
	}
	return presentation.Summary{Status: presentation.StatusError, Title: "Policy violations found"}
}

func buildsTableModel(entries []lifecycle.AuditEntry) presentation.TableModel {
	cols := []presentation.TableColumn{
		{Key: "time", Header: "TIME", MinWidth: 8, Prefer: 24, CellStyle: presentation.ValueMuted},
		{Key: "package", Header: "PACKAGE", MinWidth: 12, Prefer: 32, Primary: true, Truncate: presentation.TruncateEnd, CellStyle: presentation.ValuePackage},
		{Key: "script", Header: "SCRIPT", MinWidth: 6, Prefer: 12, CellStyle: presentation.ValueCommand},
		{Key: "exit", Header: "EXIT", MinWidth: 4, Prefer: 6, CellStyle: presentation.ValueNumber},
		{Key: "duration", Header: "MS", MinWidth: 4, Prefer: 8, CellStyle: presentation.ValueNumber},
	}
	rows := make([]map[string]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, map[string]string{
			"time":     e.TS,
			"package":  e.Package,
			"script":   e.Script,
			"exit":     strconv.Itoa(e.ExitCode),
			"duration": strconv.FormatInt(e.DurationMs, 10),
		})
	}
	return presentation.TableModel{Columns: cols, Rows: rows}
}

func verifyProvenanceSummary(subject, algo, digest string) presentation.Summary {
	return presentation.Summary{
		Status: presentation.StatusSuccess,
		Title:  "Provenance verified",
		Metrics: []presentation.KeyValue{
			{Key: "subject", Value: subject},
			{Key: "digest", Value: algo + ":" + digest, Style: presentation.ValueMuted},
		},
	}
}

func storePruneSummary(removed, kept int, dryRun bool) presentation.Summary {
	title := "Store prune complete"
	if dryRun {
		title = "Store prune dry-run"
	}
	return presentation.Summary{
		Status: presentation.StatusSuccess,
		Title:  title,
		Metrics: []presentation.KeyValue{
			{Key: "removed", Value: strconv.Itoa(removed), Style: presentation.ValueNumber},
			{Key: "kept", Value: strconv.Itoa(kept), Style: presentation.ValueNumber},
			{Key: "dry_run", Value: strconv.FormatBool(dryRun)},
		},
	}
}

func emptyNotice(msg string) presentation.Notice {
	return presentation.Notice{Status: presentation.StatusInfo, Message: msg}
}
