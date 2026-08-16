package transform_test

import (
	"bytes"
	"testing"

	"github.com/mewisme/mew/internal/transform"
)

func TestFrameRoundTrip(t *testing.T) {
	req := transform.TransformRequestV2{
		V:            transform.ProtocolVersion,
		ID:           "req-1",
		Op:           "transform",
		Path:         "src/a.ts",
		Source:       "const x: number = 1",
		SourceDigest: transform.DigestString("const x: number = 1"),
		Loader:       "ts",
		Format:       "esm",
		OptsDigest:   transform.DigestString(""),
		NodeMajor:    20,
		SourceMap:    "none",
		CancelToken:  "req-1",
	}
	var buf bytes.Buffer
	if err := transform.EncodeFrame(&buf, req); err != nil {
		t.Fatal(err)
	}
	var got transform.TransformRequestV2
	if err := transform.DecodeFrame(&buf, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != req.ID || got.Path != req.Path || got.Source != req.Source {
		t.Fatalf("got %+v want %+v", got, req)
	}

	resp := transform.TransformResponseV2{
		V:    transform.ProtocolVersion,
		ID:   "req-1",
		OK:   true,
		Code: "const x = 1;\n",
	}
	buf.Reset()
	if err := transform.EncodeFrame(&buf, resp); err != nil {
		t.Fatal(err)
	}
	var gotResp transform.TransformResponseV2
	if err := transform.DecodeFrame(&buf, &gotResp); err != nil {
		t.Fatal(err)
	}
	if gotResp.V != resp.V || gotResp.ID != resp.ID || !gotResp.OK || gotResp.Code != resp.Code {
		t.Fatalf("got %+v want %+v", gotResp, resp)
	}
}

func TestEncodeFrameLargePayload(t *testing.T) {
	// 16 MiB limit is enforced on decode, not encode.
	// Just verify encode doesn't panic on reasonable sizes.
	req := transform.TransformRequestV2{
		V:            transform.ProtocolVersion,
		ID:           "large",
		Op:           "transform",
		Path:         "big.ts",
		Source:       "const x = 1;",
		SourceDigest: transform.DigestString("const x = 1;"),
		Loader:       "ts",
		Format:       "esm",
		OptsDigest:   transform.DigestString(""),
		NodeMajor:    20,
		SourceMap:    "none",
		CancelToken:  "large",
	}
	var buf bytes.Buffer
	if err := transform.EncodeFrame(&buf, req); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Fatal("empty frame")
	}
}

func TestValidateTransformRequestV2(t *testing.T) {
	validDigest := transform.DigestString("x")
	tests := []struct {
		name    string
		req     transform.TransformRequestV2
		wantErr bool
	}{
		{
			name: "valid",
			req: transform.TransformRequestV2{
				V: transform.ProtocolVersion, ID: "1", Op: "transform",
				Path: "a.ts", Source: "x", SourceDigest: validDigest,
				Loader: "ts", Format: "esm", NodeMajor: 20,
				SourceMap: "none", CancelToken: "1", OptsDigest: transform.DigestString(""),
			},
			wantErr: false,
		},
		{
			name: "wrong version",
			req: transform.TransformRequestV2{
				V: 1, ID: "1", Op: "transform",
				Path: "a.ts", Source: "x", SourceDigest: validDigest,
				Loader: "ts", Format: "esm", NodeMajor: 20,
				SourceMap: "none", CancelToken: "1", OptsDigest: transform.DigestString(""),
			},
			wantErr: true,
		},
		{
			name: "missing path",
			req: transform.TransformRequestV2{
				V: transform.ProtocolVersion, ID: "1", Op: "transform",
				Source: "x", SourceDigest: validDigest,
				Loader: "ts", Format: "esm", NodeMajor: 20,
				SourceMap: "none", CancelToken: "1", OptsDigest: transform.DigestString(""),
			},
			wantErr: true,
		},
		{
			name: "malformed source digest",
			req: transform.TransformRequestV2{
				V: transform.ProtocolVersion, ID: "1", Op: "transform",
				Path: "a.ts", Source: "x", SourceDigest: "not-hex",
				Loader: "ts", Format: "esm", NodeMajor: 20,
				SourceMap: "none", CancelToken: "1", OptsDigest: transform.DigestString(""),
			},
			wantErr: true,
		},
		{
			name: "unknown op",
			req: transform.TransformRequestV2{
				V: transform.ProtocolVersion, ID: "1", Op: "bundle",
				Path: "a.ts", Source: "x", SourceDigest: validDigest,
				Loader: "ts", Format: "esm", NodeMajor: 20,
				SourceMap: "none", CancelToken: "1", OptsDigest: transform.DigestString(""),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestHelloRoundTrip(t *testing.T) {
	hello := transform.HelloRequest{V: transform.ProtocolVersion, Token: "secret"}
	var buf bytes.Buffer
	if err := transform.EncodeFrame(&buf, hello); err != nil {
		t.Fatal(err)
	}
	var got transform.HelloRequest
	if err := transform.DecodeFrame(&buf, &got); err != nil {
		t.Fatal(err)
	}
	if got.Token != "secret" {
		t.Fatalf("token=%q", got.Token)
	}
}

func TestHelloRequestValidateWrongVersion(t *testing.T) {
	hello := transform.HelloRequest{V: 1, Token: "secret"}
	err := hello.Validate()
	if err == nil {
		t.Fatal("expected error for wrong hello protocol version")
	}
}

func TestHelloRequestValidateOK(t *testing.T) {
	hello := transform.HelloRequest{V: transform.ProtocolVersion, Token: "secret"}
	if err := hello.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTransformRequestV2ValidateLoader(t *testing.T) {
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: transform.DigestString("x"),
		Loader: "invalid", Format: "esm", NodeMajor: 20,
		SourceMap: "none", CancelToken: "1", OptsDigest: transform.DigestString(""),
	}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for unknown loader")
	}
}

func TestTransformRequestV2ValidateFormat(t *testing.T) {
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: transform.DigestString("x"),
		Loader: "ts", Format: "umd", NodeMajor: 20,
		SourceMap: "none", CancelToken: "1", OptsDigest: transform.DigestString(""),
	}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestTransformRequestV2ValidateSourceMapMode(t *testing.T) {
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: transform.DigestString("x"),
		Loader: "ts", Format: "esm", NodeMajor: 20,
		SourceMap:   "foobar",
		CancelToken: "1", OptsDigest: transform.DigestString(""),
	}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for unknown source-map mode")
	}
}

func TestTransformRequestV2ValidateIDTooLong(t *testing.T) {
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: string(make([]byte, 300)), Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: transform.DigestString("x"),
		Loader: "ts", Format: "esm", NodeMajor: 20,
		SourceMap: "none", CancelToken: string(make([]byte, 300)), OptsDigest: transform.DigestString(""),
	}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for too-long ID")
	}
}

func TestEncodeFrameRejectsOversized(t *testing.T) {
	// Create a payload that exceeds MaxFrameSize.
	huge := transform.TransformRequestV2{
		V:            transform.ProtocolVersion,
		ID:           "1",
		Op:           "transform",
		Path:         "a.ts",
		Source:       string(make([]byte, transform.MaxFrameSize)), // > MaxFrameSize when JSON-encoded
		SourceDigest: transform.DigestString(string(make([]byte, transform.MaxFrameSize))),
		Loader:       "ts",
		Format:       "esm",
		NodeMajor:    20,
		SourceMap:    "none",
		CancelToken:  "1",
		OptsDigest:   transform.DigestString(""),
	}
	var buf bytes.Buffer
	err := transform.EncodeFrame(&buf, huge)
	if err == nil {
		t.Fatal("expected error for oversized frame")
	}
}

func TestSanitizeErrorCode(t *testing.T) {
	stable := transform.SanitizeErrorCode("ERR_M_TRANSFORM_SYNTAX")
	if stable != "ERR_M_TRANSFORM_SYNTAX" {
		t.Fatalf("stable code sanitized: %s", stable)
	}

	unknown := transform.SanitizeErrorCode("ERR_M_TOP_SECRET")
	if unknown != "ERR_M_TRANSFORM_ENGINE" {
		t.Fatalf("unknown code not sanitized: %s", unknown)
	}

	empty := transform.SanitizeErrorCode("")
	if empty != "ERR_M_TRANSFORM_ENGINE" {
		t.Fatalf("empty code not sanitized: %s", empty)
	}
}

func TestSanitizeErrorMessage(t *testing.T) {
	msg := transform.SanitizeErrorMessage("const x = 1; unexpected token")
	if msg == "const x = 1; unexpected token" {
		t.Fatal("source content not sanitized from error message")
	}

	ok := transform.SanitizeErrorMessage("transform timeout")
	if ok != "transform timeout" {
		t.Fatalf("safe message altered: %s", ok)
	}
}

func TestSanitizeErrorMessageRedactsEndpoint(t *testing.T) {
	msg := transform.SanitizeErrorMessage("dial 127.0.0.1:12345: connection refused")
	if msg == "dial 127.0.0.1:12345: connection refused" {
		t.Fatal("endpoint not sanitized")
	}
}

func TestSanitizeErrorMessageRedactsOptions(t *testing.T) {
	msg := transform.SanitizeErrorMessage(`bad option "target": "ES2022"`)
	if msg == `bad option "target": "ES2022"` {
		t.Fatal("options content not sanitized")
	}
}

func TestSanitizeErrorMessageRedactsToken(t *testing.T) {
	// A 64-char hex string looks like a token/digest.
	msg := transform.SanitizeErrorMessage("token abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789 leaked")
	if msg == "token abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789 leaked" {
		t.Fatal("hex token not sanitized")
	}
}

func TestValidateRequestHeader(t *testing.T) {
	tests := []struct {
		name    string
		v       int
		id      string
		op      string
		expect  string
		wantErr bool
	}{
		{name: "valid health", v: transform.ProtocolVersion, id: "1", op: "health", expect: "health", wantErr: false},
		{name: "valid shutdown", v: transform.ProtocolVersion, id: "2", op: "shutdown", expect: "shutdown", wantErr: false},
		{name: "wrong version", v: 1, id: "1", op: "health", expect: "health", wantErr: true},
		{name: "missing id", v: transform.ProtocolVersion, id: "", op: "health", expect: "health", wantErr: true},
		{name: "wrong op", v: transform.ProtocolVersion, id: "1", op: "health", expect: "shutdown", wantErr: true},
		{name: "id too long", v: transform.ProtocolVersion, id: string(make([]byte, 300)), op: "health", expect: "health", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := transform.ValidateRequestHeader(tt.v, tt.id, tt.op, tt.expect)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequestHeader() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestCancelRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     transform.CancelRequest
		wantErr bool
	}{
		{
			name: "valid",
			req: transform.CancelRequest{
				V: transform.ProtocolVersion, ID: "1", Op: "cancel", CancelToken: "req-1",
			},
			wantErr: false,
		},
		{
			name: "missing cancel token",
			req: transform.CancelRequest{
				V: transform.ProtocolVersion, ID: "1", Op: "cancel",
			},
			wantErr: true,
		},
		{
			name: "wrong version",
			req: transform.CancelRequest{
				V: 1, ID: "1", Op: "cancel", CancelToken: "req-1",
			},
			wantErr: true,
		},
		{
			name: "wrong op",
			req: transform.CancelRequest{
				V: transform.ProtocolVersion, ID: "1", Op: "transform", CancelToken: "req-1",
			},
			wantErr: true,
		},
		{
			name: "cancel token too long",
			req: transform.CancelRequest{
				V: transform.ProtocolVersion, ID: "1", Op: "cancel",
				CancelToken: string(make([]byte, 300)),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifySourceDigest(t *testing.T) {
	source := "const x = 1;"
	validDigest := transform.DigestString(source)

	// Valid match.
	if err := transform.VerifySourceDigest(source, validDigest); err != nil {
		t.Fatalf("valid digest rejected: %v", err)
	}

	// Mismatch.
	if err := transform.VerifySourceDigest(source, transform.DigestString("different")); err == nil {
		t.Fatal("expected mismatch error")
	}

	// Malformed.
	if err := transform.VerifySourceDigest(source, "not-hex"); err == nil {
		t.Fatal("expected malformed error")
	}
	if err := transform.VerifySourceDigest(source, "deadbeef"); err == nil {
		t.Fatal("expected malformed error for short hex")
	}
}

func TestVerifyOptionsDigest(t *testing.T) {
	opts := `{"target":"ES2022"}`
	validDigest := transform.DigestString(opts)

	// Valid match.
	if err := transform.VerifyOptionsDigest(opts, validDigest); err != nil {
		t.Fatalf("valid digest rejected: %v", err)
	}

	// Mismatch.
	if err := transform.VerifyOptionsDigest(opts, transform.DigestString("{}")); err == nil {
		t.Fatal("expected mismatch error")
	}

	// Malformed.
	if err := transform.VerifyOptionsDigest(opts, "not-hex"); err == nil {
		t.Fatal("expected malformed error")
	}
}

func TestTransformRequestV2ValidateNodeMajor(t *testing.T) {
	validDigest := transform.DigestString("x")
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: validDigest,
		Loader: "ts", Format: "esm", NodeMajor: 99,
		SourceMap: "none", CancelToken: "1", OptsDigest: transform.DigestString(""),
	}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for unsupported node major")
	}
}

func TestTransformRequestV2ValidateSourceDigestMissing(t *testing.T) {
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: "",
		Loader: "ts", Format: "esm", NodeMajor: 20,
		SourceMap: "none", CancelToken: "1", OptsDigest: transform.DigestString(""),
	}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for missing source digest")
	}
}

func TestTransformRequestV2ValidateOptsDigestMissing(t *testing.T) {
	validDigest := transform.DigestString("x")
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: validDigest,
		Options: `{"target":"ES2022"}`, OptsDigest: "",
		Loader: "ts", Format: "esm", NodeMajor: 20,
		SourceMap: "none", CancelToken: "1",
	}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for missing opts digest when options present")
	}
}

func TestTransformRequestV2ValidateOptionsLength(t *testing.T) {
	validDigest := transform.DigestString("x")
	longOpts := `"target":"ES2022",` + string(make([]byte, transform.MaxOptionsLength+1))
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: validDigest,
		Options: longOpts, OptsDigest: transform.DigestString(longOpts),
		Loader: "ts", Format: "esm", NodeMajor: 20,
		SourceMap: "none", CancelToken: "1",
	}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for too-long options")
	}
}

func TestTransformRequestV2ValidateCancelTokenLength(t *testing.T) {
	validDigest := transform.DigestString("x")
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: validDigest,
		Loader: "ts", Format: "esm", NodeMajor: 20,
		SourceMap: "none", OptsDigest: transform.DigestString(""),
		CancelToken: string(make([]byte, 300)),
	}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for too-long cancel token")
	}
}

func TestDigestString(t *testing.T) {
	d := transform.DigestString("hello")
	if len(d) != 64 {
		t.Fatalf("digest length=%d, want 64", len(d))
	}
	// Deterministic.
	if d != transform.DigestString("hello") {
		t.Fatal("digest not deterministic")
	}
}

// ── Strict validation negative tests ────────────────────────────────

func TestStrictUnmarshalRejectsUnknownFields(t *testing.T) {
	// A valid transform request with an extra unknown field.
	body := []byte(`{"v":2,"id":"1","op":"transform","path":"a.ts","source":"x","source_digest":"` +
		transform.DigestString("x") + `","loader":"ts","format":"esm","opts_digest":"` +
		transform.DigestString("") + `","node_major":20,"source_map":"none","cancel_token":"1","extra_field":"should_fail"}`)
	var req transform.TransformRequestV2
	err := transform.StrictUnmarshal(body, &req)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestStrictUnmarshalRejectsTrailingJSON(t *testing.T) {
	body := []byte(`{"v":2,"id":"1","op":"transform","path":"a.ts","source":"x","source_digest":"` +
		transform.DigestString("x") + `","loader":"ts","format":"esm","opts_digest":"` +
		transform.DigestString("") + `","node_major":20,"source_map":"none","cancel_token":"1"}{"extra":"garbage"}`)
	var req transform.TransformRequestV2
	err := transform.StrictUnmarshal(body, &req)
	if err == nil {
		t.Fatal("expected error for trailing JSON")
	}
}

func TestStrictUnmarshalRejectsUnknownFieldOnCancel(t *testing.T) {
	// Cancel request with a field that belongs to transform.
	body := []byte(`{"v":2,"id":"c1","op":"cancel","cancel_token":"tok","path":"/etc/passwd"}`)
	var req transform.CancelRequest
	err := transform.StrictUnmarshal(body, &req)
	if err == nil {
		t.Fatal("expected error for path field on cancel request")
	}
}

func TestStrictUnmarshalRejectsUnknownFieldOnShutdown(t *testing.T) {
	body := []byte(`{"v":2,"id":"s1","op":"shutdown","source":"evil"}`)
	var req transform.ShutdownRequest
	err := transform.StrictUnmarshal(body, &req)
	if err == nil {
		t.Fatal("expected error for source field on shutdown request")
	}
}

func TestStrictUnmarshalRejectsUnknownFieldOnHealth(t *testing.T) {
	body := []byte(`{"v":2,"id":"h1","op":"health","loader":"ts"}`)
	var req transform.HealthRequest
	err := transform.StrictUnmarshal(body, &req)
	if err == nil {
		t.Fatal("expected error for loader field on health request")
	}
}

func TestReadFrameBodyRejectsOversized(t *testing.T) {
	// Create a frame header claiming a payload larger than MaxFrameSize.
	var buf bytes.Buffer
	hdr := [4]byte{0xff, 0xff, 0xff, 0xff} // ~4 GiB
	buf.Write(hdr[:])
	_, err := transform.ReadFrameBody(&buf)
	if err == nil {
		t.Fatal("expected error for oversized frame body")
	}
}

func TestPeekOp(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		wantOp  string
		wantErr bool
	}{
		{name: "transform", body: []byte(`{"op":"transform"}`), wantOp: "transform", wantErr: false},
		{name: "cancel", body: []byte(`{"op":"cancel"}`), wantOp: "cancel", wantErr: false},
		{name: "shutdown", body: []byte(`{"op":"shutdown"}`), wantOp: "shutdown", wantErr: false},
		{name: "health", body: []byte(`{"op":"health"}`), wantOp: "health", wantErr: false},
		{name: "missing op", body: []byte(`{"v":2}`), wantOp: "", wantErr: false},
		{name: "malformed", body: []byte(`not json`), wantOp: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, err := transform.PeekOp(tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("PeekOp() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if op != tt.wantOp {
				t.Errorf("PeekOp() = %q, want %q", op, tt.wantOp)
			}
		})
	}
}

func TestTransformRequestV2Validate_MissingSourceMap(t *testing.T) {
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: transform.DigestString("x"),
		Loader: "ts", Format: "esm", OptsDigest: transform.DigestString(""),
		NodeMajor: 20, SourceMap: "", CancelToken: "1",
	}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for missing source-map mode")
	}
}

func TestTransformRequestV2Validate_MissingCancelToken(t *testing.T) {
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: transform.DigestString("x"),
		Loader: "ts", Format: "esm", OptsDigest: transform.DigestString(""),
		NodeMajor: 20, SourceMap: "none", CancelToken: "",
	}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for missing cancel token")
	}
}

func TestTransformRequestV2Validate_MissingOptsDigest(t *testing.T) {
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: transform.DigestString("x"),
		Loader: "ts", Format: "esm", OptsDigest: "",
		NodeMajor: 20, SourceMap: "none", CancelToken: "1",
	}
	err := req.Validate()
	if err == nil {
		t.Fatal("expected error for missing opts digest")
	}
}

func TestTransformRequestV2Validate_EmptyOptionsStillRequiresDigest(t *testing.T) {
	req := transform.TransformRequestV2{
		V: transform.ProtocolVersion, ID: "1", Op: "transform",
		Path: "a.ts", Source: "x", SourceDigest: transform.DigestString("x"),
		Loader: "ts", Format: "esm",
		Options: "", OptsDigest: transform.DigestString(""), // empty options, valid digest
		NodeMajor: 20, SourceMap: "none", CancelToken: "1",
	}
	err := req.Validate()
	if err != nil {
		t.Fatalf("empty options with valid digest should pass: %v", err)
	}
}

func TestHealthRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     transform.HealthRequest
		wantErr bool
	}{
		{
			name:    "valid",
			req:     transform.HealthRequest{V: transform.ProtocolVersion, ID: "1", Op: "health"},
			wantErr: false,
		},
		{
			name:    "wrong version",
			req:     transform.HealthRequest{V: 1, ID: "1", Op: "health"},
			wantErr: true,
		},
		{
			name:    "missing id",
			req:     transform.HealthRequest{V: transform.ProtocolVersion, Op: "health"},
			wantErr: true,
		},
		{
			name:    "wrong op",
			req:     transform.HealthRequest{V: transform.ProtocolVersion, ID: "1", Op: "transform"},
			wantErr: true,
		},
		{
			name:    "id too long",
			req:     transform.HealthRequest{V: transform.ProtocolVersion, ID: string(make([]byte, 300)), Op: "health"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestShutdownRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     transform.ShutdownRequest
		wantErr bool
	}{
		{
			name:    "valid",
			req:     transform.ShutdownRequest{V: transform.ProtocolVersion, ID: "1", Op: "shutdown"},
			wantErr: false,
		},
		{
			name:    "wrong version",
			req:     transform.ShutdownRequest{V: 1, ID: "1", Op: "shutdown"},
			wantErr: true,
		},
		{
			name:    "missing id",
			req:     transform.ShutdownRequest{V: transform.ProtocolVersion, Op: "shutdown"},
			wantErr: true,
		},
		{
			name:    "wrong op",
			req:     transform.ShutdownRequest{V: transform.ProtocolVersion, ID: "1", Op: "health"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
