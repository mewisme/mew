// Package support provides redacted support-bundle collection for bug reports.
//
// Collectors gather diagnostic metadata from production state into explicitly
// allowlisted, support-safe DTOs. Every entry is redacted before serialization;
// no raw struct, credential, env value, source file, or private path escapes.
//
// The bundle is a deterministic gzipped tar archive containing versioned JSON
// entries and a machine-readable manifest. Archives are written atomically:
// partial artifacts are never left at the destination path on failure.
package support
