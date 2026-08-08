package runtime

import (
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
