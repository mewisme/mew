package trace_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/trace"
)

func TestRedactBearer(t *testing.T) {
	input := "Authorization: Bearer abcdef1234567890abcdef1234567890"
	got := trace.Redact(input)
	if strings.Contains(got, "abcdef") {
		t.Errorf("bearer token not redacted: %s", got)
	}
	if !strings.Contains(got, "Bearer ***") {
		t.Errorf("expected Bearer *** placeholder: %s", got)
	}
}

func TestRedactQueryToken(t *testing.T) {
	cases := []string{
		"http://example.com?access_token=secret123",
		"http://example.com?authToken=secret456",
		"http://example.com?token=secret789",
		"http://example.com?password=hunter2",
	}
	for _, c := range cases {
		got := trace.Redact(c)
		if strings.Contains(got, "secret") || strings.Contains(got, "hunter2") {
			t.Errorf("query token not redacted in %q: %s", c, got)
		}
	}
}

func TestRedactEnvSecret(t *testing.T) {
	cases := []string{
		"NPM_TOKEN=secret123",
		"MY_PASSWORD=hunter2",
		"API_KEY=abcdef",
		"AWS_SECRET=key123",
	}
	for _, c := range cases {
		got := trace.Redact(c)
		if strings.Contains(got, "secret123") || strings.Contains(got, "hunter2") ||
			strings.Contains(got, "abcdef") || strings.Contains(got, "key123") {
			t.Errorf("env secret not redacted in %q: %s", c, got)
		}
	}
}

func TestRedactURL(t *testing.T) {
	input := "https://user:pass@example.com/path"
	got := trace.Redact(input)
	if strings.Contains(got, "user:pass") || strings.Contains(got, "pass@") {
		t.Errorf("URL password not redacted: %s", got)
	}
}

func TestRedactEmpty(t *testing.T) {
	if s := trace.Redact(""); s != "" {
		t.Errorf("Redact(empty) = %q", s)
	}
}

func TestRedactError(t *testing.T) {
	err := errors.New("auth failed: Bearer secret123")
	got := trace.RedactError(err)
	if got == "" {
		t.Fatal("RedactError returned empty")
	}
	if strings.Contains(got, "secret123") {
		t.Errorf("error message not redacted: %s", got)
	}
	if !strings.Contains(got, "Bearer ***") {
		t.Errorf("expected redacted bearer: %s", got)
	}
}

func TestRedactErrorNil(t *testing.T) {
	if s := trace.RedactError(nil); s != "" {
		t.Errorf("RedactError(nil) = %q, want empty", s)
	}
}

func TestSafeFilePath(t *testing.T) {
	cases := []struct {
		target, base, want string
	}{
		{"/home/user/project/src/app.ts", "/home/user/project", "src/app.ts"},
		{"/usr/bin/node", "/home/user/project", "/usr/bin/node"},
		{"", "/base", ""},
		{"/path", "", "/path"},
	}
	for _, c := range cases {
		got := trace.SafeFilePath(c.target, c.base)
		if got != c.want {
			t.Errorf("SafeFilePath(%q, %q) = %q, want %q", c.target, c.base, got, c.want)
		}
	}
}
