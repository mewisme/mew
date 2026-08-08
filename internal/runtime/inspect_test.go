package runtime

import (
	"testing"
)

func TestParseInspectorFlagsNone(t *testing.T) {
	cfg, others, err := ParseInspectorFlags([]string{"--trace-warnings", "app.js"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Fatal("expected nil config when no inspector flags present")
	}
	if len(others) != 2 || others[0] != "--trace-warnings" || others[1] != "app.js" {
		t.Fatalf("others=%v, want [--trace-warnings app.js]", others)
	}
}

func TestParseInspectorFlagsBare(t *testing.T) {
	cfg, others, err := ParseInspectorFlags([]string{"--inspect", "app.js"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if cfg.Mode != InspectorRun {
		t.Fatalf("mode=%s, want run", cfg.Mode)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("host=%s, want 127.0.0.1", cfg.Host)
	}
	if cfg.Port != 0 {
		t.Fatalf("port=%d, want 0", cfg.Port)
	}
	if cfg.ExplicitBind {
		t.Fatal("expected ExplicitBind=false for bare --inspect")
	}
	if len(others) != 1 {
		t.Fatalf("others=%v, want [app.js]", others)
	}
}

func TestParseInspectorFlagsBareBrk(t *testing.T) {
	cfg, _, err := ParseInspectorFlags([]string{"--inspect-brk", "app.ts"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if cfg.Mode != InspectorBrk {
		t.Fatalf("mode=%s, want brk", cfg.Mode)
	}
}

func TestParseInspectorFlagsWithPort(t *testing.T) {
	cfg, _, err := ParseInspectorFlags([]string{"--inspect=9229", "app.js"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if cfg.Port != 9229 {
		t.Fatalf("port=%d, want 9229", cfg.Port)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("host=%s, want 127.0.0.1", cfg.Host)
	}
	if !cfg.ExplicitBind {
		t.Fatal("expected ExplicitBind=true for explicit port")
	}
}

func TestParseInspectorFlagsWithHostPort(t *testing.T) {
	cfg, _, err := ParseInspectorFlags([]string{"--inspect=127.0.0.1:9229", "app.js"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 9229 {
		t.Fatalf("port=%d, want 9229", cfg.Port)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("host=%s, want 127.0.0.1", cfg.Host)
	}
	if !cfg.ExplicitBind {
		t.Fatal("expected ExplicitBind=true")
	}
}

func TestParseInspectorFlagsBothRunAndBrk(t *testing.T) {
	// --inspect + --inspect-brk: brk wins for mode.
	cfg, others, err := ParseInspectorFlags([]string{"--inspect", "--inspect-brk", "app.js"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != InspectorBrk {
		t.Fatalf("mode=%s, want brk", cfg.Mode)
	}
	if len(others) != 1 || others[0] != "app.js" {
		t.Fatalf("others=%v", others)
	}
}

func TestParseInspectorFlagsDuplicateSameValue(t *testing.T) {
	// Duplicate with same value is fine (Node ignores duplicates).
	_, _, err := ParseInspectorFlags([]string{"--inspect=9229", "--inspect=9229", "app.js"}, false)
	if err != nil {
		t.Fatalf("unexpected error for duplicate same-value: %v", err)
	}
}

func TestParseInspectorFlagsDuplicateDifferentValue(t *testing.T) {
	// Duplicate with different values is an error.
	_, _, err := ParseInspectorFlags([]string{"--inspect=9229", "--inspect=9230", "app.js"}, false)
	if err == nil {
		t.Fatal("expected error for conflicting --inspect values")
	}
}

func TestParseInspectorFlagsInvalidPort(t *testing.T) {
	_, _, err := ParseInspectorFlags([]string{"--inspect=99999", "app.js"}, false)
	if err == nil {
		t.Fatal("expected error for port out of range")
	}
}

func TestParseInspectorFlagsZeroPort(t *testing.T) {
	// Port 0 is valid (Node chooses ephemeral).
	cfg, _, err := ParseInspectorFlags([]string{"--inspect=0", "app.js"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 0 {
		t.Fatalf("port=%d, want 0", cfg.Port)
	}
	if !cfg.ExplicitPort {
		t.Fatal("expected ExplicitPort=true for --inspect=0")
	}
	if !cfg.ExplicitBind {
		t.Fatal("expected ExplicitBind=true for --inspect=0")
	}
}

func TestParseInspectorFlagsInspectBrkZeroPort(t *testing.T) {
	cfg, _, err := ParseInspectorFlags([]string{"--inspect-brk=0", "app.js"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != InspectorBrk {
		t.Fatalf("mode=%s, want brk", cfg.Mode)
	}
	if cfg.Port != 0 {
		t.Fatalf("port=%d, want 0", cfg.Port)
	}
	if !cfg.ExplicitPort {
		t.Fatal("expected ExplicitPort=true for --inspect-brk=0")
	}
}

func TestParseInspectorFlagsRemoteBind(t *testing.T) {
	// Non-loopback bind without env var should fail.
	_, _, err := ParseInspectorFlags([]string{"--inspect=0.0.0.0:9229", "app.js"}, false)
	if err == nil {
		t.Fatal("expected error for non-loopback bind without opt-in")
	}
}

func TestParseInspectorFlagsRemoteBindWithOptIn(t *testing.T) {
	t.Setenv("MEW_EXPERIMENTAL_REMOTE_INSPECTOR", "1")
	cfg, _, err := ParseInspectorFlags([]string{"--inspect=0.0.0.0:9229", "app.js"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "0.0.0.0" {
		t.Fatalf("host=%s, want 0.0.0.0", cfg.Host)
	}
	if cfg.Port != 9229 {
		t.Fatalf("port=%d, want 9229", cfg.Port)
	}
}

func TestParseInspectorFlagsNodeModeRemoteBind(t *testing.T) {
	// In --node mode (zeroAugmentation), remote binds are allowed without opt-in.
	cfg, _, err := ParseInspectorFlags([]string{"--inspect=0.0.0.0:9229", "app.js"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "0.0.0.0" {
		t.Fatalf("host=%s, want 0.0.0.0", cfg.Host)
	}
}

func TestParseInspectorFlagsLocalhost(t *testing.T) {
	cfg, _, err := ParseInspectorFlags([]string{"--inspect=localhost:9229", "app.js"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "localhost" {
		t.Fatalf("host=%s, want localhost", cfg.Host)
	}
}

func TestParseInspectorFlagsIPv6Loopback(t *testing.T) {
	cfg, _, err := ParseInspectorFlags([]string{"--inspect=::1:9229", "app.js"}, false)
	if err != nil {
		t.Fatal(err)
	}
	// ::1 is loopback, so it's allowed without opt-in.
	if cfg.Host != "::1" {
		t.Fatalf("host=%s, want ::1", cfg.Host)
	}
}

func TestParseInspectorFlagsEmptyArgs(t *testing.T) {
	cfg, others, err := ParseInspectorFlags(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Fatal("expected nil config for empty args")
	}
	if others != nil {
		t.Fatal("expected nil others")
	}
}

func TestParseInspectorFlagsMalformedHost(t *testing.T) {
	// Host with space should be rejected.
	_, _, err := ParseInspectorFlags([]string{"--inspect=bad host:9229", "app.js"}, false)
	if err == nil {
		t.Fatal("expected error for malformed host")
	}
}

func TestBuildInspectorArgvBare(t *testing.T) {
	cfg := &InspectorConfig{Mode: InspectorRun, Host: "127.0.0.1", Port: 0, ExplicitBind: false, ExplicitPort: false}
	argv := cfg.BuildInspectorArgv()
	if len(argv) != 1 || argv[0] != "--inspect" {
		t.Fatalf("argv=%v, want [--inspect]", argv)
	}
}

func TestBuildInspectorArgvWithPort(t *testing.T) {
	cfg := &InspectorConfig{Mode: InspectorRun, Host: "127.0.0.1", Port: 9229, ExplicitBind: true, ExplicitPort: true}
	argv := cfg.BuildInspectorArgv()
	if len(argv) != 1 || argv[0] != "--inspect=127.0.0.1:9229" {
		t.Fatalf("argv=%v, want [--inspect=127.0.0.1:9229]", argv)
	}
}

func TestBuildInspectorArgvBrk(t *testing.T) {
	cfg := &InspectorConfig{Mode: InspectorBrk, Host: "127.0.0.1", Port: 0, ExplicitBind: false, ExplicitPort: false}
	argv := cfg.BuildInspectorArgv()
	if len(argv) != 1 || argv[0] != "--inspect-brk" {
		t.Fatalf("argv=%v, want [--inspect-brk]", argv)
	}
}

func TestBuildInspectorArgvRemote(t *testing.T) {
	cfg := &InspectorConfig{Mode: InspectorRun, Host: "0.0.0.0", Port: 9230, ExplicitBind: true, ExplicitPort: true}
	argv := cfg.BuildInspectorArgv()
	if len(argv) != 1 || argv[0] != "--inspect=0.0.0.0:9230" {
		t.Fatalf("argv=%v, want [--inspect=0.0.0.0:9230]", argv)
	}
}

func TestBuildInspectorArgvZeroPort(t *testing.T) {
	// Explicit port 0 must be preserved in output.
	cfg := &InspectorConfig{Mode: InspectorRun, Host: "127.0.0.1", Port: 0, ExplicitBind: true, ExplicitPort: true}
	argv := cfg.BuildInspectorArgv()
	if len(argv) != 1 || argv[0] != "--inspect=127.0.0.1:0" {
		t.Fatalf("argv=%v, want [--inspect=127.0.0.1:0]", argv)
	}
}

func TestBuildInspectorArgvZeroPortBrk(t *testing.T) {
	cfg := &InspectorConfig{Mode: InspectorBrk, Host: "127.0.0.1", Port: 0, ExplicitBind: true, ExplicitPort: true}
	argv := cfg.BuildInspectorArgv()
	if len(argv) != 1 || argv[0] != "--inspect-brk=127.0.0.1:0" {
		t.Fatalf("argv=%v, want [--inspect-brk=127.0.0.1:0]", argv)
	}
}

func TestBuildInspectorArgvHostOnly(t *testing.T) {
	// Explicit host, default port.
	cfg := &InspectorConfig{Mode: InspectorRun, Host: "0.0.0.0", Port: 0, ExplicitBind: true, ExplicitPort: false}
	argv := cfg.BuildInspectorArgv()
	if len(argv) != 1 || argv[0] != "--inspect=0.0.0.0" {
		t.Fatalf("argv=%v, want [--inspect=0.0.0.0]", argv)
	}
}

func TestBuildInspectorArgvIPv6(t *testing.T) {
	// IPv6 must use bracket notation in output.
	cfg := &InspectorConfig{Mode: InspectorRun, Host: "::1", Port: 9229, ExplicitBind: true, ExplicitPort: true}
	argv := cfg.BuildInspectorArgv()
	if len(argv) != 1 || argv[0] != "--inspect=[::1]:9229" {
		t.Fatalf("argv=%v, want [--inspect=[::1]:9229]", argv)
	}
}

func TestBuildInspectorArgvIPv6DefaultPort(t *testing.T) {
	cfg := &InspectorConfig{Mode: InspectorRun, Host: "::1", Port: 0, ExplicitBind: true, ExplicitPort: false}
	argv := cfg.BuildInspectorArgv()
	if len(argv) != 1 || argv[0] != "--inspect=[::1]" {
		t.Fatalf("argv=%v, want [--inspect=[::1]]", argv)
	}
}

func TestBuildInspectorArgvNil(t *testing.T) {
	var cfg *InspectorConfig
	if argv := cfg.BuildInspectorArgv(); argv != nil {
		t.Fatalf("argv=%v, want nil", argv)
	}
}

func TestIsLoopback(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"localhost", true},
		{"0.0.0.0", false},
		{"::", false},
		{"192.168.1.1", false},
	}
	for _, tt := range tests {
		got := isLoopback(tt.host)
		if got != tt.want {
			t.Errorf("isLoopback(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}

func TestParseInspectorAddr(t *testing.T) {
	tests := []struct {
		raw              string
		wantHost         string
		wantPort         int
		wantExplicit     bool
		wantExplicitPort bool
		wantErr          bool
	}{
		{"", "127.0.0.1", 0, false, false, false},
		{"9229", "127.0.0.1", 9229, true, true, false},
		{"0", "127.0.0.1", 0, true, true, false},
		{":9229", "127.0.0.1", 9229, true, true, false},
		{":0", "127.0.0.1", 0, true, true, false},
		{"127.0.0.1:9229", "127.0.0.1", 9229, true, true, false},
		{"127.0.0.1:0", "127.0.0.1", 0, true, true, false},
		{"0.0.0.0:9229", "0.0.0.0", 9229, true, true, false},
		{"localhost:9229", "localhost", 9229, true, true, false},
		{"::1:9229", "::1", 9229, true, true, false},
		{"::1:0", "::1", 0, true, true, false},
		// Bracketed IPv6.
		{"[::1]:9229", "::1", 9229, true, true, false},
		{"[::1]:0", "::1", 0, true, true, false},
		{"[::1]", "::1", 0, true, false, false},
		{"[::]:9229", "::", 9229, true, true, false},
		// Host only (no port).
		{"127.0.0.1", "127.0.0.1", 0, true, false, false},
		{"0.0.0.0", "0.0.0.0", 0, true, false, false},
		// Errors.
		{"99999", "", 0, false, false, true},
		{"127.0.0.1:99999", "", 0, false, false, true},
		{"[::1", "", 0, false, false, true},       // unmatched bracket
		{"[::1]:", "", 0, false, false, true},      // trailing colon after bracket
		{"[::1]garbage", "", 0, false, false, true}, // garbage after bracket
	}
	for _, tt := range tests {
		host, port, explicit, explicitPort, err := parseInspectorAddr(tt.raw)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseInspectorAddr(%q): expected error", tt.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseInspectorAddr(%q): unexpected error: %v", tt.raw, err)
			continue
		}
		if host != tt.wantHost {
			t.Errorf("parseInspectorAddr(%q).host = %q, want %q", tt.raw, host, tt.wantHost)
		}
		if port != tt.wantPort {
			t.Errorf("parseInspectorAddr(%q).port = %d, want %d", tt.raw, port, tt.wantPort)
		}
		if explicit != tt.wantExplicit {
			t.Errorf("parseInspectorAddr(%q).explicit = %v, want %v", tt.raw, explicit, tt.wantExplicit)
		}
		if explicitPort != tt.wantExplicitPort {
			t.Errorf("parseInspectorAddr(%q).explicitPort = %v, want %v", tt.raw, explicitPort, tt.wantExplicitPort)
		}
	}
}
