package presentation

import "github.com/mewisme/mew/internal/apperr"

// TitleForCode returns a stable human title for an error code.
func TitleForCode(code apperr.Code) string {
	if title, ok := errorTitles[code]; ok {
		return title
	}
	return "Operation failed"
}

var errorTitles = map[apperr.Code]string{
	apperr.OK:                       "Success",
	apperr.Usage:                    "Invalid command usage",
	apperr.Cancelled:                "Operation cancelled",
	apperr.Internal:                 "Mew encountered an internal error",
	apperr.InternalPanic:            "Mew encountered an internal error",
	apperr.IO:                       "Filesystem operation failed",
	apperr.Config:                   "Configuration is invalid",
	apperr.Network:                  "Network operation failed",
	apperr.Integrity:                "Integrity verification failed",
	apperr.Lockfile:                 "Lockfile validation failed",
	apperr.LockUnsupported:          "Lockfile format is not supported",
	apperr.LockAmbiguous:            "Lockfile producer version is ambiguous",
	apperr.LockUnrepresentable:      "Lockfile cannot be represented safely",
	apperr.Unimplemented:            "Command is not implemented",
	apperr.Unsupported:              "Operation is not supported",
	apperr.Manifest:                 "Package manifest is invalid",
	apperr.NotFound:                 "Required item was not found",
	apperr.Resolve:                  "Dependency resolution failed",
	apperr.Install:                  "Installation failed",
	apperr.Transaction:              "Project update failed",
	apperr.Store:                    "Content store operation failed",
	apperr.Policy:                   "Operation blocked by policy",
	apperr.PNPUnsupported:           "PnP install is not supported",
	apperr.Exec:                     "Command execution failed",
	apperr.Timeout:                  "Operation timed out",
	apperr.RuntimeNodeNotFound:      "Node.js was not found",
	apperr.RuntimeNodeVersion:       "Node.js version is not supported",
	apperr.RuntimeNodeUnsupported:   "Node.js installation is not supported",
	apperr.RuntimeEntrypoint:        "Invalid runtime entrypoint",
	apperr.RuntimeInvocation:        "Node.js process invocation failed",
	apperr.RuntimeAssetManifest:     "Runtime asset manifest is invalid",
	apperr.RuntimeAssetDigest:       "Runtime asset integrity check failed",
	apperr.RuntimeAssetExtract:      "Runtime asset extraction failed",
	apperr.RuntimeAssetCache:        "Runtime asset cache is corrupt",
	apperr.RuntimeNodeStart:         "Node.js process failed to start",
	apperr.ChildExit:                "Child process exited with error",
	apperr.TransformSyntax:          "TypeScript syntax error",
	apperr.TransformUnsupported:     "Unsupported TypeScript syntax",
	apperr.TransformConfigParse:     "Invalid tsconfig file",
	apperr.TransformConfigExtends:   "Tsconfig extends chain is invalid",
	apperr.TransformConfigOption:    "Unsupported tsconfig option",
	apperr.TransformProtocolVersion: "Transform protocol version mismatch",
	apperr.TransformAuth:            "Transform service authentication failed",
	apperr.TransformFrameSize:       "Transform frame exceeds size limit",
	apperr.TransformTimeout:         "Transform request timed out",
	apperr.TransformCancelled:       "Transform request was cancelled",
	apperr.TransformUnavailable:     "Transform service is not available",
	apperr.TransformCacheCorrupt:    "Transform cache is corrupt",
	apperr.TransformEngine:          "Transform engine internal failure",
	apperr.EnvFileNotFound:          "Environment file not found",
	apperr.EnvFileRead:              "Environment file is not readable",
	apperr.EnvFileParse:             "Environment file is malformed",
	apperr.InspectorBind:            "Inspector failed to bind to address",
	apperr.InspectorPort:            "Inspector port is not valid",
	apperr.InspectorHost:            "Inspector host is not valid",
	apperr.InspectorDup:             "Duplicate inspector flag",
}
