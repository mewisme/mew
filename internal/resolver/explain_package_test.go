package resolver_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mewisme/mew/internal/policy"
	"github.com/mewisme/mew/internal/resolver"
	"github.com/mewisme/mew/internal/testkit"
)

func readExplainFixture(t testing.TB, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testkit.FixtureDir(t, rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestExplainPackageOverrideChain(t *testing.T) {
	eng, _ := engineWithPackuments(t, overridePackuments())
	root := writeProject(t, readExplainFixture(t, "explain/override-chain/package.json"))
	ex, err := eng.ExplainPackage(context.Background(), root, "pkg-b", resolver.ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if ex == nil || len(ex.Decisions) != 1 {
		t.Fatalf("decisions=%#v", ex)
	}
	d := ex.Decisions[0]
	if d.Selected != "1.0.0" || d.Reason != "tag-or-exact" {
		t.Fatalf("decision=%#v", d)
	}
	if d.OverrideFrom == "" {
		t.Fatalf("expected override decision: %#v", d)
	}
	if len(ex.Paths) == 0 {
		t.Fatal("expected import paths")
	}
}

func TestExplainPackagePeerConflict(t *testing.T) {
	eng, _ := engineWithPackuments(t, reactPackuments())
	root := writeProject(t, readExplainFixture(t, "explain/peer-conflict/package.json"))
	ex, err := eng.ExplainPackage(context.Background(), root, "react-dom", resolver.ResolveOptions{
		Policy: &policy.Policy{StrictPeerDependencies: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ex == nil || ex.Conflict == nil {
		t.Fatal("expected peer conflict explanation")
	}
	if ex.Conflict.Peer != "react-dom" {
		t.Fatalf("peer=%q", ex.Conflict.Peer)
	}
}

func TestExplainPackageNotFound(t *testing.T) {
	eng, _ := engineWithPackuments(t, overridePackuments())
	root := writeProject(t, readExplainFixture(t, "explain/override-chain/package.json"))
	_, err := eng.ExplainPackage(context.Background(), root, "missing-pkg", resolver.ResolveOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReasonDetailForKnownReasons(t *testing.T) {
	for _, reason := range []string{"reuse-key", "hint", "max-satisfying", "workspace"} {
		detail := resolver.ReasonDetailFor(reason)
		if detail.Text == "" {
			t.Fatalf("reason=%q detail=%#v", reason, detail)
		}
	}
	if detail := resolver.ReasonDetailFor("platform-skipped"); detail.Code == "" {
		t.Fatalf("expected resolve code: %#v", detail)
	}
}

func TestFormatPackageExplanationOverrideChain(t *testing.T) {
	eng, _ := engineWithPackuments(t, overridePackuments())
	root := writeProject(t, readExplainFixture(t, "explain/override-chain/package.json"))
	ex, err := eng.ExplainPackage(context.Background(), root, "pkg-b", resolver.ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := resolver.FormatPackageExplanation(ex, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"package pkg-b", "pkg-b@", "imported by:", "pkg-a@1.0.0"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}
