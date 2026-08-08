// Package apperr defines typed Mew errors with stable ERR_M_* codes.
package apperr

import (
	"errors"
	"fmt"
)

// Code is a stable machine-readable error identifier.
type Code string

const (
	OK                     Code = "ERR_M_OK"
	Usage                  Code = "ERR_M_USAGE"
	Cancelled              Code = "ERR_M_CANCELLED"
	Internal               Code = "ERR_M_INTERNAL"
	InternalPanic          Code = "ERR_M_INTERNAL_PANIC"
	IO                     Code = "ERR_M_IO"
	Config                 Code = "ERR_M_CONFIG"
	Network                Code = "ERR_M_NETWORK"
	Integrity              Code = "ERR_M_INTEGRITY"
	Lockfile               Code = "ERR_M_LOCKFILE"
	LockUnsupported        Code = "ERR_M_LOCK_UNSUPPORTED"
	LockAmbiguous          Code = "ERR_M_LOCK_AMBIGUOUS"
	LockUnrepresentable    Code = "ERR_M_LOCK_UNREPRESENTABLE"
	Unimplemented          Code = "ERR_M_UNIMPLEMENTED"
	Unsupported            Code = "ERR_M_UNSUPPORTED"
	Manifest               Code = "ERR_M_MANIFEST"
	NotFound               Code = "ERR_M_NOT_FOUND"
	Resolve                Code = "ERR_M_RESOLVE"
	Install                Code = "ERR_M_INSTALL"
	Transaction            Code = "ERR_M_TRANSACTION"
	Store                  Code = "ERR_M_STORE"
	Policy                 Code = "ERR_M_POLICY"
	PNPUnsupported         Code = "ERR_M_PNP_UNSUPPORTED"
	Exec                   Code = "ERR_M_EXEC"
	Timeout                Code = "ERR_M_TIMEOUT"
	RuntimeNodeNotFound    Code = "ERR_M_RUNTIME_NODE_NOT_FOUND"
	RuntimeNodeVersion     Code = "ERR_M_RUNTIME_NODE_VERSION"
	RuntimeNodeUnsupported Code = "ERR_M_RUNTIME_NODE_UNSUPPORTED"
	RuntimeEntrypoint      Code = "ERR_M_RUNTIME_ENTRYPOINT"
	RuntimeInvocation      Code = "ERR_M_RUNTIME_INVOCATION"
	RuntimeAssetManifest   Code = "ERR_M_RUNTIME_ASSET_MANIFEST"
	RuntimeAssetDigest     Code = "ERR_M_RUNTIME_ASSET_DIGEST"
	RuntimeAssetExtract    Code = "ERR_M_RUNTIME_ASSET_EXTRACTION"
	RuntimeAssetCache      Code = "ERR_M_RUNTIME_ASSET_CACHE"
	RuntimeNodeStart       Code = "ERR_M_RUNTIME_NODE_START"
	ChildExit              Code = "ERR_M_CHILD_EXIT"
	// Transform error codes (0051).
	TransformSyntax          Code = "ERR_M_TRANSFORM_SYNTAX"
	TransformUnsupported     Code = "ERR_M_TRANSFORM_UNSUPPORTED"
	TransformConfigParse     Code = "ERR_M_TRANSFORM_CONFIG_PARSE"
	TransformConfigExtends   Code = "ERR_M_TRANSFORM_CONFIG_EXTENDS"
	TransformConfigOption    Code = "ERR_M_TRANSFORM_CONFIG_OPTION"
	TransformProtocolVersion Code = "ERR_M_TRANSFORM_PROTOCOL_VERSION"
	TransformAuth            Code = "ERR_M_TRANSFORM_AUTH"
	TransformFrameSize       Code = "ERR_M_TRANSFORM_FRAME_SIZE"
	TransformTimeout         Code = "ERR_M_TRANSFORM_TIMEOUT"
	TransformCancelled       Code = "ERR_M_TRANSFORM_CANCELLED"
	TransformUnavailable     Code = "ERR_M_TRANSFORM_UNAVAILABLE"
	TransformCacheCorrupt    Code = "ERR_M_TRANSFORM_CACHE_CORRUPT"
	TransformEngine          Code = "ERR_M_TRANSFORM_ENGINE"
	// Env file error codes (0054).
	EnvFileNotFound Code = "ERR_M_ENV_FILE_NOT_FOUND"
	EnvFileRead     Code = "ERR_M_ENV_FILE_READ"
	EnvFileParse    Code = "ERR_M_ENV_FILE_PARSE"
	// Inspector error codes (0056).
	InspectorBind Code = "ERR_M_INSPECTOR_BIND"
	InspectorPort Code = "ERR_M_INSPECTOR_PORT"
	InspectorHost Code = "ERR_M_INSPECTOR_HOST"
	InspectorDup  Code = "ERR_M_INSPECTOR_DUPLICATE"
)

// registry maps every published code to a process exit status.
var registry = map[Code]int{
	OK:                       0,
	Usage:                    2,
	Cancelled:                130,
	Internal:                 1,
	InternalPanic:            1,
	IO:                       1,
	Config:                   1,
	Network:                  1,
	Integrity:                1,
	Lockfile:                 1,
	LockUnsupported:          1,
	LockAmbiguous:            1,
	LockUnrepresentable:      1,
	Unimplemented:            1,
	Unsupported:              1,
	Manifest:                 1,
	NotFound:                 1,
	Resolve:                  1,
	Install:                  1,
	Transaction:              1,
	Store:                    1,
	Policy:                   1,
	PNPUnsupported:           1,
	Exec:                     1,
	Timeout:                  1,
	RuntimeNodeNotFound:      1,
	RuntimeNodeVersion:       1,
	RuntimeNodeUnsupported:   1,
	RuntimeEntrypoint:        1,
	RuntimeInvocation:        1,
	RuntimeAssetManifest:     1,
	RuntimeAssetDigest:       1,
	RuntimeAssetExtract:      1,
	RuntimeAssetCache:        1,
	RuntimeNodeStart:         1,
	ChildExit:                1, // exit code taken from ExitStatus.ExitCode(), not registry
	TransformSyntax:          1,
	TransformUnsupported:     1,
	TransformConfigParse:     1,
	TransformConfigExtends:   1,
	TransformConfigOption:    1,
	TransformProtocolVersion: 1,
	TransformAuth:            1,
	TransformFrameSize:       1,
	TransformTimeout:         1,
	TransformCancelled:       130,
	TransformUnavailable:     1,
	TransformCacheCorrupt:    1,
	TransformEngine:          1,
	EnvFileNotFound:          1,
	EnvFileRead:              1,
	EnvFileParse:             1,
	InspectorBind:            1,
	InspectorPort:            1,
	InspectorHost:            1,
	InspectorDup:             1,
}

// AllCodes returns registered codes in a stable order for docs and tests.
func AllCodes() []Code {
	return []Code{
		OK, Usage, Cancelled, Internal, InternalPanic,
		IO, Config, Network, Integrity, Lockfile, LockUnsupported, LockAmbiguous, LockUnrepresentable, Unimplemented, Unsupported,
		Manifest, NotFound, Resolve, Install, Transaction, Store, Policy, PNPUnsupported, Exec, Timeout,
		RuntimeNodeNotFound, RuntimeNodeVersion, RuntimeNodeUnsupported,
		RuntimeEntrypoint, RuntimeInvocation,
		RuntimeAssetManifest, RuntimeAssetDigest, RuntimeAssetExtract, RuntimeAssetCache,
		RuntimeNodeStart,
		ChildExit,
		TransformSyntax, TransformUnsupported,
		TransformConfigParse, TransformConfigExtends, TransformConfigOption,
		TransformProtocolVersion, TransformAuth, TransformFrameSize,
		TransformTimeout, TransformCancelled, TransformUnavailable,
		TransformCacheCorrupt, TransformEngine,
		EnvFileNotFound, EnvFileRead, EnvFileParse,
		InspectorBind, InspectorPort, InspectorHost, InspectorDup,
	}
}

// ExitForCode returns the exit status for a registered code, or 1 if unknown.
func ExitForCode(c Code) int {
	if n, ok := registry[c]; ok {
		return n
	}
	return 1
}

// Error is a typed application error.
type Error struct {
	Code     Code
	Op       string
	Subject  string
	Message  string
	Cause    error
	ExitHint int // 0 = use Code mapping
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	msg := e.Message
	if msg == "" && e.Cause != nil {
		msg = e.Cause.Error()
	}
	if e.Op != "" && e.Subject != "" {
		return fmt.Sprintf("%s: %s: %s: %s", e.Code, e.Op, e.Subject, msg)
	}
	if e.Op != "" {
		return fmt.Sprintf("%s: %s: %s", e.Code, e.Op, msg)
	}
	return fmt.Sprintf("%s: %s", e.Code, msg)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// ExitStatus is a child process non-zero exit. It is NOT an internal Mew failure.<br>// CLI handlers must preserve the child exit code rather than formatting it as ERR_M_INTERNAL.
type ExitStatus struct {
	Code int
	Err  error
}

func (e *ExitStatus) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

func (e *ExitStatus) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ExitCode extracts an exit code from err when available.
func (e *ExitStatus) ExitCode() int {
	if e == nil {
		return 0
	}
	return e.Code
}

// New constructs a typed error without a cause.
func New(code Code, op, subject, msg string) *Error {
	return &Error{Code: code, Op: op, Subject: subject, Message: msg}
}

// Wrap constructs a typed error wrapping cause.
func Wrap(code Code, op, subject string, err error) *Error {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return &Error{Code: code, Op: op, Subject: subject, Message: msg, Cause: err}
}

// CodeOf extracts the first apperr.Code in the chain, or Internal if none.
// OperationFailure resolves through Primary before falling back to Internal.
func CodeOf(err error) Code {
	var of *OperationFailure
	if errors.As(err, &of) && of != nil && of.Primary != nil {
		err = of.Primary
	}
	var ae *Error
	if errors.As(err, &ae) && ae != nil && ae.Code != "" {
		return ae.Code
	}
	var es *ExitStatus
	if errors.As(err, &es) && es != nil {
		return ChildExit
	}
	return Internal
}

// ExitCode maps an error to a process exit status.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var es *ExitStatus
	if errors.As(err, &es) && es != nil {
		return es.ExitCode()
	}
	var ae *Error
	if errors.As(err, &ae) && ae != nil {
		if ae.ExitHint != 0 {
			return ae.ExitHint
		}
		return ExitForCode(ae.Code)
	}
	return 1
}

// IsUsage reports whether err is a usage / invalid-arguments failure.
func IsUsage(err error) bool {
	return CodeOf(err) == Usage
}
