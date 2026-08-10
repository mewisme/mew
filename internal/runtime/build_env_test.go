package runtime

import (
	"strings"
	"testing"
)

func TestBuildEnvHostPrecedence(t *testing.T) {
	// Host env vars should not be overridden by dotenv overlay.
	t.Setenv("EXISTING", "host_value")
	t.Setenv("OVERRIDE_ME", "host_override")

	envOverlay := []string{"EXISTING=new", "NEW_VAR=from_dotenv"}
	planChanges := []string{"RUNTIME_VAR=from_plan"}

	result := buildEnv(envOverlay, planChanges)
	m := environToMap(result)

	// Host values must win over dotenv overlay.
	if m["EXISTING"] != "host_value" {
		t.Errorf("EXISTING = %q, want host_value (host must win over dotenv)", m["EXISTING"])
	}
	// Dotenv can set vars absent from host.
	if m["NEW_VAR"] != "from_dotenv" {
		t.Errorf("NEW_VAR = %q, want from_dotenv", m["NEW_VAR"])
	}
	// Plan changes always apply (internal runtime needs).
	if m["RUNTIME_VAR"] != "from_plan" {
		t.Errorf("RUNTIME_VAR = %q, want from_plan", m["RUNTIME_VAR"])
	}
	// Plan changes override host too.
	if m["OVERRIDE_ME"] != "host_override" {
		t.Errorf("OVERRIDE_ME = %q, want host_override", m["OVERRIDE_ME"])
	}
}

func TestBuildEnvPlanOverridesHost(t *testing.T) {
	// Plan env changes must still override host env (internal runtime requirements).
	t.Setenv("MEW_USER_LOADERS", "user_value")

	planChanges := []string{"MEW_USER_LOADERS=runtime_value"}
	result := buildEnv(nil, planChanges)
	m := environToMap(result)

	if m["MEW_USER_LOADERS"] != "runtime_value" {
		t.Errorf("MEW_USER_LOADERS = %q, want runtime_value (plan must override host)", m["MEW_USER_LOADERS"])
	}
}

func TestBuildEnvNoOverlayReturnsHost(t *testing.T) {
	t.Setenv("TEST_VAR", "test")
	result := buildEnv(nil, nil)
	m := environToMap(result)
	if m["TEST_VAR"] != "test" {
		t.Errorf("TEST_VAR = %q, want test", m["TEST_VAR"])
	}
}

// -- Windows normalization tests (portable: use buildEnvOS with goos="windows") --

// Windows: host Path casing variant not overridden by dotenv PATH.
func TestBuildEnvWindowsHostPrecedenceCasing(t *testing.T) {
	base := []string{
		"Path=C:\\host",
		"NODE_ENV=production",
	}
	dotenv := []string{
		"PATH=C:\\dotenv",
		"node_env=dev",
		"FOO=bar",
	}
	plan := []string{
		"MEW_TOKEN=x",
	}

	result := runBuildEnvOS(base, dotenv, plan, "windows")
	lu := newWinLookup(result)

	if v, ok := lu.get("PATH"); !ok || v != `C:\host` {
		t.Errorf("PATH = %q, want C:\\host (host must win over dotenv)", v)
	}
	if v, ok := lu.get("NODE_ENV"); !ok || v != "production" {
		t.Errorf("NODE_ENV = %q, want production (host must win over dotenv)", v)
	}
	if v, ok := lu.get("FOO"); !ok || v != "bar" {
		t.Errorf("FOO = %q, want bar (dotenv fills absent key)", v)
	}
	if v, ok := lu.get("MEW_TOKEN"); !ok || v != "x" {
		t.Errorf("MEW_TOKEN = %q, want x (plan always applies)", v)
	}
}

// Windows: plan path overrides host Path and produces exactly one logical PATH.
func TestBuildEnvWindowsPlanOverridesHostPath(t *testing.T) {
	base := []string{
		"Path=C:\\host",
		"SystemRoot=C:\\Windows",
	}
	plan := []string{
		"path=C:\\runtime",
	}

	result := runBuildEnvOS(base, nil, plan, "windows")
	lu := newWinLookup(result)

	if v, ok := lu.get("PATH"); !ok || v != `C:\runtime` {
		t.Errorf("PATH = %q, want C:\\runtime (plan must override host)", v)
	}
	if n := countLogicalKey(result, "PATH"); n != 1 {
		t.Errorf("logical PATH entries = %d, want 1", n)
	}
	if v, ok := lu.get("SystemRoot"); !ok || v != `C:\Windows` {
		t.Errorf("SystemRoot = %q, want C:\\Windows (unaffected host key)", v)
	}
}

// Windows: plan casing variant of MEW_* replaces the host variant.
func TestBuildEnvWindowsPlanMEWSecurityCasing(t *testing.T) {
	base := []string{
		"mew_user_loaders=C:\\host\\evil",
		"MEW_LOCAL_STORAGE_PATH=C:\\host\\storage",
	}
	plan := []string{
		"MEW_USER_LOADERS=C:\\runtime\\safe",
	}

	result := runBuildEnvOS(base, nil, plan, "windows")
	lu := newWinLookup(result)

	if v, ok := lu.get("MEW_USER_LOADERS"); !ok || v != `C:\runtime\safe` {
		t.Errorf("MEW_USER_LOADERS = %q, want C:\\runtime\\safe (plan must override host casing variant)", v)
	}
	if n := countLogicalKey(result, "MEW_USER_LOADERS"); n != 1 {
		t.Errorf("logical MEW_USER_LOADERS entries = %d, want 1", n)
	}
	if v, ok := lu.get("MEW_LOCAL_STORAGE_PATH"); !ok || v != `C:\host\storage` {
		t.Errorf("MEW_LOCAL_STORAGE_PATH = %q, want C:\\host\\storage (host value preserved)", v)
	}
}

// Windows: multiple host entries differing only by case are normalized deterministically.
func TestBuildEnvWindowsDuplicateHostNormalization(t *testing.T) {
	base := []string{
		"Path=C:\\first",
		"PATH=C:\\second",
		"path=C:\\third",
	}

	result := runBuildEnvOS(base, nil, nil, "windows")

	// Should have at most one logical PATH entry.
	if n := countLogicalKey(result, "PATH"); n > 1 {
		t.Errorf("logical PATH entries = %d, want <= 1", n)
	}
	// First-wins: the first entry (Path) should be the one kept.
	lu := newWinLookup(result)
	if v, ok := lu.get("PATH"); !ok || v != `C:\first` {
		t.Errorf("PATH = %q, want C:\\first (first host entry wins for duplicates)", v)
	}
}

// Windows: empty value overrides work correctly.
func TestBuildEnvWindowsEmptyValueOverride(t *testing.T) {
	base := []string{
		"Path=C:\\host",
	}
	plan := []string{
		"PATH=",
	}

	result := runBuildEnvOS(base, nil, plan, "windows")
	lu := newWinLookup(result)

	if v, ok := lu.get("PATH"); !ok || v != "" {
		t.Errorf("PATH = %q, want empty (plan empty value must override host)", v)
	}
}

// Unix: FOO and foo remain distinct.
func TestBuildEnvUnixCaseSensitive(t *testing.T) {
	base := []string{
		"FOO=upper",
		"foo=lower",
	}

	result := runBuildEnvOS(base, nil, nil, "linux")
	m := environToMap(result)

	if m["FOO"] != "upper" {
		t.Errorf("FOO = %q, want upper", m["FOO"])
	}
	if m["foo"] != "lower" {
		t.Errorf("foo = %q, want lower", m["foo"])
	}
}

// Windows: full precedence chain — host > dotenv, plan > all.
func TestBuildEnvWindowsFullPrecedenceChain(t *testing.T) {
	base := []string{
		"Path=C:\\host",
		"NODE_ENV=production",
	}
	dotenv := []string{
		"PATH=C:\\dotenv",
		"node_env=dev",
		"FOO=bar",
	}
	plan := []string{
		"path=C:\\runtime",
		"MEW_TOKEN=x",
	}

	result := runBuildEnvOS(base, dotenv, plan, "windows")
	lu := newWinLookup(result)

	// Plan overrides both host and dotenv for PATH.
	if v, ok := lu.get("PATH"); !ok || v != `C:\runtime` {
		t.Errorf("PATH = %q, want C:\\runtime (plan overrides host)", v)
	}
	if n := countLogicalKey(result, "PATH"); n != 1 {
		t.Errorf("logical PATH entries = %d, want 1", n)
	}
	// Host protects NODE_ENV from dotenv.
	if v, ok := lu.get("NODE_ENV"); !ok || v != "production" {
		t.Errorf("NODE_ENV = %q, want production (host wins over dotenv)", v)
	}
	// FOO from dotenv (absent from host and plan).
	if v, ok := lu.get("FOO"); !ok || v != "bar" {
		t.Errorf("FOO = %q, want bar (dotenv fills absent)", v)
	}
	// MEW_TOKEN from plan.
	if v, ok := lu.get("MEW_TOKEN"); !ok || v != "x" {
		t.Errorf("MEW_TOKEN = %q, want x", v)
	}
}

// Security: casing variant cannot bypass replacement of sensitive internal variables.
func TestBuildEnvWindowsSecurityBypassPrevention(t *testing.T) {
	base := []string{
		"Path=C:\\evil",
		"NODE_OPTIONS=--inspect-brk",
	}
	plan := []string{
		"NODE_OPTIONS=",
		"path=C:\\safe",
	}

	result := runBuildEnvOS(base, nil, plan, "windows")
	lu := newWinLookup(result)

	if v, ok := lu.get("NODE_OPTIONS"); !ok || v != "" {
		t.Errorf("NODE_OPTIONS = %q, want empty (plan must clear security-sensitive var regardless of casing)", v)
	}
	if v, ok := lu.get("PATH"); !ok || v != `C:\safe` {
		t.Errorf("PATH = %q, want C:\\safe", v)
	}
	if n := countLogicalKey(result, "NODE_OPTIONS"); n != 1 {
		t.Errorf("logical NODE_OPTIONS entries = %d, want 1", n)
	}
	if n := countLogicalKey(result, "PATH"); n != 1 {
		t.Errorf("logical PATH entries = %d, want 1", n)
	}
}

// -- test helpers --

func environToMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				m[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	return m
}

// runBuildEnvOS calls buildEnvOS with a synthetic base environment via the test hook.
func runBuildEnvOS(base, dotenv, plan []string, goos string) []string {
	prev := testHookBaseEnv
	testHookBaseEnv = func() []string { return base }
	defer func() { testHookBaseEnv = prev }()
	return buildEnvOS(dotenv, plan, goos)
}

// winLookup performs case-insensitive key lookup in an env slice.
type winLookup struct {
	m map[string]string // normalized -> value
}

func newWinLookup(env []string) *winLookup {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		key, val, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		m[strings.ToUpper(key)] = val
	}
	return &winLookup{m: m}
}

func (w *winLookup) get(key string) (string, bool) {
	v, ok := w.m[strings.ToUpper(key)]
	return v, ok
}

// countLogicalKey counts entries matching key case-insensitively.
func countLogicalKey(env []string, key string) int {
	n := 0
	normalized := strings.ToUpper(key)
	for _, kv := range env {
		k, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if strings.ToUpper(k) == normalized {
			n++
		}
	}
	return n
}
