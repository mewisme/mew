package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/config"
	"github.com/mewisme/mew/internal/jsonfile"
)

func TestParsePhaseAGlobalsAndForwarding(t *testing.T) {
	phase, err := ParsePhaseA([]string{"--cwd", "./app", "build", "--mode", "production"})
	if err != nil {
		t.Fatal(err)
	}
	if phase.Selector != "build" {
		t.Fatalf("selector=%q", phase.Selector)
	}
	if phase.Leading.cwd != "./app" {
		t.Fatalf("cwd=%q", phase.Leading.cwd)
	}
	if got := strings.Join(phase.ForwardedArgs, ","); got != "--mode,production" {
		t.Fatalf("forwarded=%v", phase.ForwardedArgs)
	}

	phase, err = ParsePhaseA([]string{"--cwd=./app", "build", "--", "--mode", "production"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(phase.ForwardedArgs, ","); got != "--mode,production" {
		t.Fatalf("forwarded=%v", phase.ForwardedArgs)
	}

	phase, err = ParsePhaseA([]string{"-r", "--workspace-concurrency", "2", "build"})
	if err != nil {
		t.Fatal(err)
	}
	if !phase.Leading.recursive || phase.Leading.wsConcurrency != 2 {
		t.Fatalf("leading=%+v", phase.Leading)
	}
	if phase.Selector != "build" {
		t.Fatalf("selector=%q", phase.Selector)
	}

	phase, err = ParsePhaseA([]string{"-r", "build", "--workspace-concurrency", "2"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(phase.ForwardedArgs, ","); got != "--workspace-concurrency,2" {
		t.Fatalf("forwarded=%v", phase.ForwardedArgs)
	}
}

func TestParsePhaseAUnknownFlag(t *testing.T) {
	_, err := ParsePhaseA([]string{"--not-a-mew-flag", "dev"})
	if err == nil || apperr.CodeOf(err) != apperr.Usage {
		t.Fatalf("err=%v", err)
	}
}

func TestForwardedScriptArgsParity(t *testing.T) {
	args := []string{"build", "--mode", "production"}
	if got := forwardedScriptArgs(args, -1); strings.Join(got, ",") != "--mode,production" {
		t.Fatalf("got=%v", got)
	}
	args = []string{"build", "--", "--mode", "production"}
	if got := forwardedScriptArgs(args, 1); strings.Join(got, ",") != "--mode,production" {
		t.Fatalf("got=%v", got)
	}
}

func TestDirectScriptsGate(t *testing.T) {
	t.Setenv("MEW_EXPERIMENTAL_DIRECT_SCRIPTS", "")
	eff := &config.Effective{Values: map[string]config.Value{}}
	if DirectScriptsEnabled(eff) {
		t.Fatal("expected disabled")
	}
	t.Setenv("MEW_EXPERIMENTAL_DIRECT_SCRIPTS", "1")
	if !DirectScriptsEnabled(eff) {
		t.Fatal("expected env enabled")
	}
	t.Setenv("MEW_EXPERIMENTAL_DIRECT_SCRIPTS", "")
	eff.Values["runner.direct_scripts.enabled"] = config.Value{Raw: true, Source: config.SourceProject}
	if !DirectScriptsEnabled(eff) {
		t.Fatal("expected config enabled")
	}
}

func TestReservedDriftAgainstCobra(t *testing.T) {
	root := NewMRoot(testBuildInfo())
	if missing := driftAgainstShippedBuiltins(root); len(missing) > 0 {
		t.Fatalf("cobra tree missing shipped names: %v", missing)
	}
}

func TestLeadingGlobalParserDrift(t *testing.T) {
	root := NewMRoot(testBuildInfo())
	got := phaseAParserFlagNames(root)
	for _, name := range rootPersistentFlagNames(root) {
		if !containsString(got, name) {
			t.Fatalf("phase A parser missing root flag %q in %v", name, got)
		}
	}
}

func TestBuiltinBeatsScript(t *testing.T) {
	projDir := t.TempDir()
	pkg := map[string]any{
		"name":    "demo",
		"version": "1.0.0",
		"scripts": map[string]string{"install": "echo nope"},
	}
	raw, _ := jsonfile.Marshal(pkg)
	if err := os.WriteFile(filepath.Join(projDir, "package.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	root := NewMRoot(testBuildInfo())
	t.Setenv("MEW_EXPERIMENTAL_DIRECT_SCRIPTS", "1")
	phase, err := ParsePhaseA([]string{"install"})
	if err != nil {
		t.Fatal(err)
	}
	res := ResolveDispatch(root, phase, projDir, &config.Effective{Values: map[string]config.Value{
		"runner.direct_scripts.enabled": {Raw: true},
	}})
	if res.Kind != OutcomeBuiltin {
		t.Fatalf("kind=%s", res.Kind)
	}
}

func TestExactScriptCaseSensitive(t *testing.T) {
	projDir := t.TempDir()
	pkg := map[string]any{
		"name":    "demo",
		"version": "1.0.0",
		"scripts": map[string]string{"dev": "echo dev"},
	}
	raw, _ := jsonfile.Marshal(pkg)
	if err := os.WriteFile(filepath.Join(projDir, "package.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	root := NewMRoot(testBuildInfo())
	eff := &config.Effective{Values: map[string]config.Value{
		"runner.direct_scripts.enabled": {Raw: true},
	}}
	phase := PhaseAResult{Selector: "Dev"}
	res := ResolveDispatch(root, phase, projDir, eff)
	if res.Kind == OutcomeScript {
		t.Fatal("case mismatch must not execute")
	}
	phase.Selector = "dev"
	res = ResolveDispatch(root, phase, projDir, eff)
	if res.Kind != OutcomeScript {
		t.Fatalf("kind=%s", res.Kind)
	}
}

func TestGateOffExactScriptMessage(t *testing.T) {
	t.Setenv("MEW_EXPERIMENTAL_DIRECT_SCRIPTS", "")
	projDir := t.TempDir()
	pkg := map[string]any{
		"name":    "demo",
		"version": "1.0.0",
		"scripts": map[string]string{"dev": "echo dev"},
	}
	raw, _ := jsonfile.Marshal(pkg)
	if err := os.WriteFile(filepath.Join(projDir, "package.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	root := NewMRoot(testBuildInfo())
	phase := PhaseAResult{Selector: "dev"}
	res := ResolveDispatch(root, phase, projDir, &config.Effective{Values: map[string]config.Value{}})
	if res.Kind != OutcomeSuggest {
		t.Fatalf("kind=%s", res.Kind)
	}
	if res.Err == nil || !strings.Contains(res.Err.Error(), "Direct script shortcuts are disabled") {
		t.Fatalf("err=%v", res.Err)
	}
	if !strings.Contains(res.Err.Error(), "m run dev") {
		t.Fatalf("err=%v", res.Err)
	}
}

func TestBuiltinTypoOutsideProject(t *testing.T) {
	root := NewMRoot(testBuildInfo())
	phase := PhaseAResult{Selector: "instal"}
	res := ResolveDispatch(root, phase, t.TempDir(), nil)
	if res.Kind != OutcomeSuggest {
		t.Fatalf("kind=%s", res.Kind)
	}
	foundInstall := false
	for _, s := range res.Suggestions {
		if s.Name == "install" {
			foundInstall = true
		}
	}
	if !foundInstall {
		t.Fatalf("suggestions=%v", res.Suggestions)
	}
}

func TestSuggestionRankingAndLimit(t *testing.T) {
	candidates := [][]Suggestion{
		{{Name: "dev", Kind: DispatchScript, Invocation: "m run dev", Distance: 1}},
		{{Name: "install", Kind: DispatchBuiltin, Invocation: "m install", Distance: 2}},
		{{Name: "i", Kind: DispatchAlias, Invocation: "m i", Distance: 2}},
	}
	got := mergeSuggestions(candidates...)
	if len(got) > 3 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Kind != DispatchBuiltin {
		t.Fatalf("first=%+v", got[0])
	}
}

func TestDispatchJSONBuiltin(t *testing.T) {
	root := NewMRoot(testBuildInfo())
	res := ResolveDispatch(root, PhaseAResult{Selector: "install"}, "", nil)
	raw, err := encodeDispatchJSON(res, "install")
	if err != nil {
		t.Fatal(err)
	}
	var doc dispatchJSON
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.SchemaVersion != 1 || doc.Kind != "builtin" || doc.Path != "install" {
		t.Fatalf("%+v", doc)
	}
}

func TestIsPlausibleScriptSelector(t *testing.T) {
	if !isPlausibleScriptSelector("dev") {
		t.Fatal("dev should be plausible")
	}
	if isPlausibleScriptSelector("not-a-command") {
		t.Fatal("arbitrary phrase should not be plausible")
	}
}

func TestReservedScriptSuggestionUsesRun(t *testing.T) {
	s := formatScriptInvocation("add", true, true)
	if s != "m run add" {
		t.Fatalf("got %q", s)
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestScriptWinsOverFile(t *testing.T) {
	// exact package scripts beat bare file selectors per documented dispatch precedence.
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	t.Setenv("MEW_EXPERIMENTAL_DIRECT_SCRIPTS", "1")

	projDir := t.TempDir()

	pkg := map[string]any{
		"name":    "demo",
		"version": "1.0.0",
		"scripts": map[string]string{"app.js": "echo script"},
	}
	raw, err := jsonfile.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "package.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	// create a conflicting JS file on disk
	if err := os.WriteFile(filepath.Join(projDir, "app.js"), []byte("console.log(1);"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := NewMRoot(testBuildInfo())
	eff := &config.Effective{Values: map[string]config.Value{
		"runner.direct_scripts.enabled": {Raw: true},
	}}
	phase := PhaseAResult{Selector: "app.js"}
	res := ResolveDispatch(root, phase, projDir, eff)
	if res.Kind != OutcomeScript {
		t.Fatalf("kind=%s, want script (exact script must beat file-run)", res.Kind)
	}
}

func TestParsePhaseANodeFlag(t *testing.T) {
	phase, err := ParsePhaseA([]string{"--node", "app.js"})
	if err != nil {
		t.Fatal(err)
	}
	if !phase.Leading.node {
		t.Fatal("expected --node flag to be parsed")
	}
	if phase.Selector != "app.js" {
		t.Fatalf("selector=%q, want app.js", phase.Selector)
	}
}

func TestParsePhaseAInspectFlag(t *testing.T) {
	// Bare --inspect
	phase, err := ParsePhaseA([]string{"--inspect", "app.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if len(phase.Leading.v8Args) != 1 || phase.Leading.v8Args[0] != "--inspect" {
		t.Fatalf("v8Args=%v, want [--inspect]", phase.Leading.v8Args)
	}
	if phase.Selector != "app.ts" {
		t.Fatalf("selector=%q, want app.ts", phase.Selector)
	}
}

func TestParsePhaseAInspectBrkFlag(t *testing.T) {
	// Bare --inspect-brk
	phase, err := ParsePhaseA([]string{"--inspect-brk", "app.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if len(phase.Leading.v8Args) != 1 || phase.Leading.v8Args[0] != "--inspect-brk" {
		t.Fatalf("v8Args=%v, want [--inspect-brk]", phase.Leading.v8Args)
	}
	if phase.Selector != "app.ts" {
		t.Fatalf("selector=%q, want app.ts", phase.Selector)
	}
}

func TestParsePhaseAInspectWithPort(t *testing.T) {
	// --inspect=127.0.0.1:9229
	phase, err := ParsePhaseA([]string{"--inspect=127.0.0.1:9229", "app.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if len(phase.Leading.v8Args) != 1 || phase.Leading.v8Args[0] != "--inspect=127.0.0.1:9229" {
		t.Fatalf("v8Args=%v, want [--inspect=127.0.0.1:9229]", phase.Leading.v8Args)
	}
	if phase.Selector != "app.ts" {
		t.Fatalf("selector=%q, want app.ts", phase.Selector)
	}
}

func TestParsePhaseAInspectBrkWithPort(t *testing.T) {
	// --inspect-brk=0.0.0.0:9230
	phase, err := ParsePhaseA([]string{"--inspect-brk=0.0.0.0:9230", "app.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if len(phase.Leading.v8Args) != 1 || phase.Leading.v8Args[0] != "--inspect-brk=0.0.0.0:9230" {
		t.Fatalf("v8Args=%v, want [--inspect-brk=0.0.0.0:9230]", phase.Leading.v8Args)
	}
}

func TestParsePhaseACombinedV8Flags(t *testing.T) {
	// Multiple V8 flags combined
	phase, err := ParsePhaseA([]string{"--inspect", "--inspect-brk=9229", "--node", "app.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if len(phase.Leading.v8Args) != 2 {
		t.Fatalf("v8Args=%v, want 2 entries", phase.Leading.v8Args)
	}
	if phase.Leading.v8Args[0] != "--inspect" {
		t.Fatalf("v8Args[0]=%q, want --inspect", phase.Leading.v8Args[0])
	}
	if phase.Leading.v8Args[1] != "--inspect-brk=9229" {
		t.Fatalf("v8Args[1]=%q, want --inspect-brk=9229", phase.Leading.v8Args[1])
	}
	if !phase.Leading.node {
		t.Fatal("expected --node flag to be parsed")
	}
}

func TestRuntimeGate(t *testing.T) {
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "")
	if RuntimeEnabled() {
		t.Fatal("expected disabled")
	}
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	if !RuntimeEnabled() {
		t.Fatal("expected enabled")
	}
}

func TestFileRunDispatch(t *testing.T) {
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	projDir := t.TempDir()

	// create a JS file
	jsPath := projDir + "/app.js"
	if err := os.WriteFile(jsPath, []byte("console.log(1);"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := NewMRoot(testBuildInfo())
	eff := &config.Effective{Values: map[string]config.Value{}}

	phase := PhaseAResult{Selector: "app.js"}
	res := ResolveDispatch(root, phase, projDir, eff)
	if res.Kind != OutcomeFileRun {
		t.Fatalf("kind=%s, want fileRun", res.Kind)
	}
	if res.FileRunPath == "" {
		t.Fatal("expected FileRunPath to be set")
	}
}

func TestFileRunDispatchDisabled(t *testing.T) {
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "")
	projDir := t.TempDir()

	jsPath := projDir + "/app.js"
	if err := os.WriteFile(jsPath, []byte("console.log(1);"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := NewMRoot(testBuildInfo())
	eff := &config.Effective{Values: map[string]config.Value{}}

	phase := PhaseAResult{Selector: "app.js"}
	res := ResolveDispatch(root, phase, projDir, eff)
	if res.Kind == OutcomeFileRun {
		t.Fatal("fileRun dispatch should be disabled when gate is off")
	}
}

func TestFileRunDispatchMissingFile(t *testing.T) {
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	projDir := t.TempDir()
	root := NewMRoot(testBuildInfo())
	eff := &config.Effective{Values: map[string]config.Value{}}

	phase := PhaseAResult{Selector: "missing.js"}
	res := ResolveDispatch(root, phase, projDir, eff)
	if res.Kind != OutcomeUnknown {
		t.Fatalf("kind=%s, want unknown", res.Kind)
	}
	if res.Err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFileRunDispatchBuiltinWins(t *testing.T) {
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	root := NewMRoot(testBuildInfo())

	// "install" is a builtin, so it should win even if install.js exists
	phase := PhaseAResult{Selector: "install"}
	res := ResolveDispatch(root, phase, t.TempDir(), &config.Effective{Values: map[string]config.Value{}})
	if res.Kind != OutcomeBuiltin {
		t.Fatalf("kind=%s, want builtin (builtin beats file-run)", res.Kind)
	}
}

func TestDispatchJSONFileRun(t *testing.T) {
	t.Setenv("MEW_EXPERIMENTAL_RUNTIME", "1")
	projDir := t.TempDir()

	jsPath := projDir + "/app.js"
	if err := os.WriteFile(jsPath, []byte("console.log(1);"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := NewMRoot(testBuildInfo())
	res := ResolveDispatch(root, PhaseAResult{Selector: "app.js"}, projDir, &config.Effective{Values: map[string]config.Value{}})
	raw, err := encodeDispatchJSON(res, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	var doc dispatchJSON
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Kind != "fileRun" {
		t.Fatalf("kind=%s, want fileRun", doc.Kind)
	}
}

func TestParsePhaseABooleanFalseValues(t *testing.T) {
	// Phase A parser must match Cobra semantics: --flag=false must set flag to false.
	tests := []struct {
		args  []string
		check func(leadingDispatchFlags) bool
		name  string
	}{
		{
			args:  []string{"--debug=false", "build"},
			check: func(l leadingDispatchFlags) bool { return !l.debug },
			name:  "debug=false",
		},
		{
			args:  []string{"--debug=true", "build"},
			check: func(l leadingDispatchFlags) bool { return l.debug },
			name:  "debug=true",
		},
		{
			args:  []string{"--debug", "build"},
			check: func(l leadingDispatchFlags) bool { return l.debug },
			name:  "debug bare (true)",
		},
		{
			args:  []string{"--offline=false", "build"},
			check: func(l leadingDispatchFlags) bool { return !l.offline },
			name:  "offline=false",
		},
		{
			args:  []string{"--offline=0", "build"},
			check: func(l leadingDispatchFlags) bool { return !l.offline },
			name:  "offline=0",
		},
		{
			args:  []string{"--offline=no", "build"},
			check: func(l leadingDispatchFlags) bool { return !l.offline },
			name:  "offline=no",
		},
		{
			args:  []string{"--recursive=false", "build"},
			check: func(l leadingDispatchFlags) bool { return !l.recursive },
			name:  "recursive=false",
		},
		{
			args:  []string{"--if-present=false", "build"},
			check: func(l leadingDispatchFlags) bool { return !l.ifPresent },
			name:  "if-present=false",
		},
		{
			args:  []string{"--node=false", "build"},
			check: func(l leadingDispatchFlags) bool { return !l.node },
			name:  "node=false",
		},
		{
			args:  []string{"--workspace-bail=false", "build"},
			check: func(l leadingDispatchFlags) bool { return !l.wsBail },
			name:  "workspace-bail=false",
		},
		{
			args:  []string{"--prefer-offline=false", "build"},
			check: func(l leadingDispatchFlags) bool { return !l.preferOffline },
			name:  "prefer-offline=false",
		},
		{
			args:  []string{"--no-color=false", "build"},
			check: func(l leadingDispatchFlags) bool { return !l.noColor },
			name:  "no-color=false",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			phase, err := ParsePhaseA(tc.args)
			if err != nil {
				t.Fatalf("ParsePhaseA(%v): %v", tc.args, err)
			}
			if !tc.check(phase.Leading) {
				t.Fatalf("unexpected leading flags for %s: %+v", tc.name, phase.Leading)
			}
		})
	}
}
