// Package transform hosts the Go transform service and IPC.
//
// Protocol: docs/architecture/transform-ipc-sketch.md
package transform

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/mewisme/mew/internal/apperr"
)

// ProtocolVersion is the current protocol version.
const ProtocolVersion = 2

// MaxFrameSize is the maximum allowed frame payload in bytes (48 MiB).
// Must be large enough to hold a MaxSourceSize source plus JSON encoding overhead.
const MaxFrameSize = 48 << 20

// MaxSourceSize is the maximum source bytes in a transform request (32 MiB).
const MaxSourceSize = 32 << 20

// MaxPathLength limits path, option, and message strings (4 KiB).
const MaxPathLength = 4096

// MaxIDLength limits request IDs (256 bytes).
const MaxIDLength = 256

// MaxCancelTokenLength limits cancel tokens (256 bytes).
const MaxCancelTokenLength = 256

// MaxOptionsLength limits the options JSON string (64 KiB).
const MaxOptionsLength = 64 << 10

// hexDigestRE matches a valid SHA-256 hex digest (64 hex chars).
var hexDigestRE = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

// containsHexDigestRE detects a 64-char hex sequence anywhere in a string.
var containsHexDigestRE = regexp.MustCompile(`[a-fA-F0-9]{64}`)

// SupportedNodeMajors lists Node.js major versions accepted for transform.
var SupportedNodeMajors = map[int]bool{
	18: true, 20: true, 22: true, 24: true,
}

// Op codes.
const (
	OpHello     = "hello"
	OpHealth    = "health"
	OpTransform = "transform"
	OpCancel    = "cancel"
	OpShutdown  = "shutdown"
)

// ValidLoaderKinds lists the loader strings accepted on the wire.
var ValidLoaderKinds = map[string]bool{
	"ts": true, "tsx": true, "mts": true, "cts": true,
}

// ValidFormats lists the format strings accepted on the wire.
var ValidFormats = map[string]bool{
	"esm": true, "cjs": true,
}

// ValidSourceMapModes lists the source-map strings accepted on the wire.
var ValidSourceMapModes = map[string]bool{
	"none": true, "inline": true, "external": true,
}

// HelloRequest carries auth on first connection.
type HelloRequest struct {
	V     int    `json:"v"`
	Token string `json:"token"`
}

// Validate checks protocol version.
func (r *HelloRequest) Validate() error {
	if r.V != ProtocolVersion {
		return apperr.New(apperr.TransformProtocolVersion, "transform.protocol", "",
			fmt.Sprintf("unsupported hello protocol version %d", r.V))
	}
	return nil
}

// HelloResponse confirms or rejects the session.
type HelloResponse struct {
	V       int    `json:"v"`
	OK      bool   `json:"ok"`
	ErrCode string `json:"err_code,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// TransformRequestV2 is the production transform request.
type TransformRequestV2 struct {
	V            int    `json:"v"`
	ID           string `json:"id"`
	Op           string `json:"op"`
	Path         string `json:"path"`
	Source       string `json:"source"`
	SourceDigest string `json:"source_digest"`
	Loader       string `json:"loader"`
	Format       string `json:"format"`
	Options      string `json:"options"` // JSON-encoded NormalizedOptions
	OptsDigest   string `json:"opts_digest"`
	NodeMajor    int    `json:"node_major"`
	SourceMap    string `json:"source_map"`   // "none", "inline", "external"
	CancelToken  string `json:"cancel_token"` // ID used by OpCancel to cancel this request
}

// Validate checks required fields, limits, enum values, and digest integrity.
func (r *TransformRequestV2) Validate() error {
	if r.V != ProtocolVersion {
		return apperr.New(apperr.TransformProtocolVersion, "transform.protocol", r.ID,
			fmt.Sprintf("unsupported protocol version %d", r.V))
	}
	if r.ID == "" {
		return apperr.New(apperr.Usage, "transform.protocol", "", "missing request id")
	}
	if len(r.ID) > MaxIDLength {
		return apperr.New(apperr.Usage, "transform.protocol", r.ID, "request id too long")
	}
	if r.Op != OpTransform {
		return apperr.New(apperr.Unsupported, "transform.protocol", r.ID,
			fmt.Sprintf("unknown op %q", r.Op))
	}
	if r.Path == "" {
		return apperr.New(apperr.Usage, "transform.protocol", r.ID, "missing path")
	}
	if len(r.Path) > MaxPathLength {
		return apperr.New(apperr.TransformFrameSize, "transform.protocol", r.ID,
			fmt.Sprintf("path too long: %d", len(r.Path)))
	}
	if len(r.Source) > MaxSourceSize {
		return apperr.New(apperr.TransformFrameSize, "transform.protocol", r.ID,
			fmt.Sprintf("source too large: %d", len(r.Source)))
	}
	if !hexDigestRE.MatchString(r.SourceDigest) {
		return apperr.New(apperr.Integrity, "transform.protocol", r.ID,
			"source_digest missing or malformed")
	}
	if len(r.Options) > MaxOptionsLength {
		return apperr.New(apperr.TransformFrameSize, "transform.protocol", r.ID,
			fmt.Sprintf("options too long: %d", len(r.Options)))
	}
	if !hexDigestRE.MatchString(r.OptsDigest) {
		return apperr.New(apperr.Integrity, "transform.protocol", r.ID,
			"opts_digest missing or malformed")
	}
	if !SupportedNodeMajors[r.NodeMajor] {
		return apperr.New(apperr.Usage, "transform.protocol", r.ID,
			fmt.Sprintf("unsupported node major %d", r.NodeMajor))
	}
	if !ValidLoaderKinds[r.Loader] {
		return apperr.New(apperr.Usage, "transform.protocol", r.ID,
			fmt.Sprintf("unknown loader %q", r.Loader))
	}
	if !ValidFormats[r.Format] {
		return apperr.New(apperr.Usage, "transform.protocol", r.ID,
			fmt.Sprintf("unknown format %q", r.Format))
	}
	if !ValidSourceMapModes[r.SourceMap] {
		return apperr.New(apperr.Usage, "transform.protocol", r.ID,
			fmt.Sprintf("unknown or missing source-map mode %q", r.SourceMap))
	}
	if r.CancelToken == "" {
		return apperr.New(apperr.Usage, "transform.protocol", r.ID, "missing cancel token")
	}
	if len(r.CancelToken) > MaxCancelTokenLength {
		return apperr.New(apperr.Usage, "transform.protocol", r.ID, "cancel token too long")
	}
	return nil
}

// TransformResponseV2 is the production transform response.
type TransformResponseV2 struct {
	V       int    `json:"v"`
	ID      string `json:"id"`
	OK      bool   `json:"ok"`
	Code    string `json:"code,omitempty"`
	Map     string `json:"map,omitempty"` // source map as string
	Digest  string `json:"digest,omitempty"`
	ErrCode string `json:"err_code,omitempty"`
	Error   string `json:"error,omitempty"`
	Cache   string `json:"cache,omitempty"` // "hit", "miss", "bypass"
}

// CancelRequest cancels an in-flight transform.
type CancelRequest struct {
	V           int    `json:"v"`
	ID          string `json:"id"`
	Op          string `json:"op"`
	CancelToken string `json:"cancel_token"`
}

// Validate checks required fields for a cancel request.
func (r *CancelRequest) Validate() error {
	if r.V != ProtocolVersion {
		return apperr.New(apperr.TransformProtocolVersion, "transform.protocol", r.ID,
			fmt.Sprintf("unsupported protocol version %d", r.V))
	}
	if r.ID == "" {
		return apperr.New(apperr.Usage, "transform.protocol", "", "missing request id")
	}
	if len(r.ID) > MaxIDLength {
		return apperr.New(apperr.Usage, "transform.protocol", r.ID, "request id too long")
	}
	if r.Op != OpCancel {
		return apperr.New(apperr.Unsupported, "transform.protocol", r.ID,
			fmt.Sprintf("unknown op %q", r.Op))
	}
	if r.CancelToken == "" {
		return apperr.New(apperr.Usage, "transform.protocol", r.ID, "missing cancel token")
	}
	if len(r.CancelToken) > MaxCancelTokenLength {
		return apperr.New(apperr.Usage, "transform.protocol", r.ID, "cancel token too long")
	}
	return nil
}

// HealthRequest is a no-op health check.
type HealthRequest struct {
	V  int    `json:"v"`
	ID string `json:"id"`
	Op string `json:"op"`
}

// Validate checks required fields for a health request.
func (r *HealthRequest) Validate() error {
	if r.V != ProtocolVersion {
		return apperr.New(apperr.TransformProtocolVersion, "transform.protocol", r.ID,
			fmt.Sprintf("unsupported protocol version %d", r.V))
	}
	if r.ID == "" {
		return apperr.New(apperr.Usage, "transform.protocol", "", "missing request id")
	}
	if len(r.ID) > MaxIDLength {
		return apperr.New(apperr.Usage, "transform.protocol", r.ID, "request id too long")
	}
	if r.Op != OpHealth {
		return apperr.New(apperr.Unsupported, "transform.protocol", r.ID,
			fmt.Sprintf("unknown op %q", r.Op))
	}
	return nil
}

// ShutdownRequest requests a graceful connection close.
type ShutdownRequest struct {
	V  int    `json:"v"`
	ID string `json:"id"`
	Op string `json:"op"`
}

// Validate checks required fields for a shutdown request.
func (r *ShutdownRequest) Validate() error {
	if r.V != ProtocolVersion {
		return apperr.New(apperr.TransformProtocolVersion, "transform.protocol", r.ID,
			fmt.Sprintf("unsupported protocol version %d", r.V))
	}
	if r.ID == "" {
		return apperr.New(apperr.Usage, "transform.protocol", "", "missing request id")
	}
	if len(r.ID) > MaxIDLength {
		return apperr.New(apperr.Usage, "transform.protocol", r.ID, "request id too long")
	}
	if r.Op != OpShutdown {
		return apperr.New(apperr.Unsupported, "transform.protocol", r.ID,
			fmt.Sprintf("unknown op %q", r.Op))
	}
	return nil
}

// EncodeFrame writes a u32le length-prefixed JSON payload.
// Rejects frames larger than MaxFrameSize before writing.
func EncodeFrame(w io.Writer, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(body) > MaxFrameSize {
		return fmt.Errorf("transform frame too large for encode: %d (max %d)", len(body), MaxFrameSize)
	}
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(body)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

// DecodeFrame reads a u32le length-prefixed JSON payload into dest.
// Rejects frames larger than MaxFrameSize before allocating body.
func DecodeFrame(r io.Reader, dest any) error {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return err
	}
	n := binary.LittleEndian.Uint32(hdr[:])
	if n > MaxFrameSize {
		return fmt.Errorf("transform frame too large: %d (max %d)", n, MaxFrameSize)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}

// ReadFrameBody reads a u32le length-prefixed frame and returns the raw body bytes.
// Rejects frames larger than MaxFrameSize before allocating.
func ReadFrameBody(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint32(hdr[:])
	if n > MaxFrameSize {
		return nil, fmt.Errorf("transform frame too large: %d (max %d)", n, MaxFrameSize)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

// StrictUnmarshal unmarshals JSON with unknown-field rejection and trailing-data
// detection. Use for all inbound request frames to enforce the exact operation schema.
func StrictUnmarshal(data []byte, dest any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return err
	}
	// Reject trailing data after the first complete JSON value.
	if dec.More() {
		return fmt.Errorf("trailing data after JSON value")
	}
	return nil
}

// PeekOp extracts the "op" field from raw frame bytes for dispatch routing.
// Uses a tolerant unmarshal (unknown fields allowed) since the full strict
// unmarshal happens later against the operation-specific type.
func PeekOp(body []byte) (string, error) {
	var probe struct {
		Op string `json:"op"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return "", err
	}
	return probe.Op, nil
}

// ValidateRequestHeader checks the fields common to all request types: version, ID, and op.
// Returns nil when the header is valid for the expected operation.
func ValidateRequestHeader(v int, id, op, expectedOp string) error {
	if v != ProtocolVersion {
		return apperr.New(apperr.TransformProtocolVersion, "transform.protocol", id,
			fmt.Sprintf("unsupported protocol version %d", v))
	}
	if id == "" {
		return apperr.New(apperr.Usage, "transform.protocol", "", "missing request id")
	}
	if len(id) > MaxIDLength {
		return apperr.New(apperr.Usage, "transform.protocol", id, "request id too long")
	}
	if op != expectedOp {
		return apperr.New(apperr.Unsupported, "transform.protocol", id,
			fmt.Sprintf("unknown op %q", op))
	}
	return nil
}

// VerifySourceDigest checks that the SHA-256 of source bytes matches the expected hex digest.
func VerifySourceDigest(source string, digest string) error {
	if !hexDigestRE.MatchString(digest) {
		return apperr.New(apperr.Integrity, "transform.protocol", "",
			"source_digest malformed")
	}
	h := sha256.Sum256([]byte(source))
	actual := hex.EncodeToString(h[:])
	if !strings.EqualFold(actual, digest) {
		return apperr.New(apperr.Integrity, "transform.protocol", "",
			"source_digest mismatch")
	}
	return nil
}

// VerifyOptionsDigest checks that the SHA-256 of the options JSON string matches the expected hex digest.
func VerifyOptionsDigest(optionsJSON string, digest string) error {
	if !hexDigestRE.MatchString(digest) {
		return apperr.New(apperr.Integrity, "transform.protocol", "",
			"opts_digest malformed")
	}
	h := sha256.Sum256([]byte(optionsJSON))
	actual := hex.EncodeToString(h[:])
	if !strings.EqualFold(actual, digest) {
		return apperr.New(apperr.Integrity, "transform.protocol", "",
			"opts_digest mismatch")
	}
	return nil
}

// DigestString returns the SHA-256 hex digest of s.
func DigestString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// stableErrorCodes lists error codes that are safe to return to clients.
// Source content, secrets, and internal details must not appear in diagnostics.
var stableErrorCodes = map[string]bool{
	string(apperr.TransformSyntax):          true,
	string(apperr.TransformUnsupported):     true,
	string(apperr.TransformConfigParse):     true,
	string(apperr.TransformConfigExtends):   true,
	string(apperr.TransformConfigOption):    true,
	string(apperr.TransformProtocolVersion): true,
	string(apperr.TransformAuth):            true,
	string(apperr.TransformFrameSize):       true,
	string(apperr.TransformTimeout):         true,
	string(apperr.TransformCancelled):       true,
	string(apperr.TransformUnavailable):     true,
	string(apperr.TransformCacheCorrupt):    true,
	string(apperr.TransformEngine):          true,
	string(apperr.Usage):                    true,
	string(apperr.Unsupported):              true,
	string(apperr.Integrity):                true,
	string(apperr.Cancelled):                true,
}

// SanitizeErrorCode returns code if it is a stable, safe-to-expose error code;
// otherwise returns the generic engine error code.
func SanitizeErrorCode(code string) string {
	if stableErrorCodes[code] {
		return code
	}
	return string(apperr.TransformEngine)
}

// SanitizeErrorMessage returns msg if it is safe to expose; strips source content,
// endpoint addresses, tokens, and options payloads.
func SanitizeErrorMessage(msg string) string {
	if len(msg) > MaxPathLength {
		msg = msg[:MaxPathLength] + "..."
	}
	// Redact source content that may have leaked into error messages.
	if strings.Contains(msg, "const ") || strings.Contains(msg, "import ") {
		return "transform error (details redacted)"
	}
	// Redact endpoint addresses.
	if strings.Contains(msg, "127.0.0.1:") || strings.Contains(msg, "localhost:") {
		return "transform error (details redacted)"
	}
	// Redact JSON options payloads.
	if strings.Contains(msg, `"target"`) || strings.Contains(msg, `"module"`) {
		return "transform error (details redacted)"
	}
	// Redact hex tokens (64-char sequences that look like digests or tokens).
	if containsHexDigestRE.MatchString(msg) {
		return "transform error (details redacted)"
	}
	return msg
}
