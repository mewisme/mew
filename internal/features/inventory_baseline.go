package features

func row(id, name, module string, nub, mew Status, class CompatibilityClass, mvp string) Feature {
	return Feature{
		ID:                 id,
		Name:               name,
		Module:             module,
		NubStatus:          nub,
		MewStatus:          mew,
		CompatibilityClass: class,
		PrimaryMVP:         mvp,
		Tests:              []string{},
	}
}

func rowWithTests(id, name, module string, nub, mew Status, class CompatibilityClass, mvp string, tests []string) Feature {
	f := row(id, name, module, nub, mew, class, mvp)
	f.Tests = tests
	return f
}

// Baseline returns the canonical feature inventory rows from plans/0002.
func Baseline() *Inventory {
	return &Inventory{
		SchemaVersion: "1",
		Features:      baselineFeatures(),
	}
}

func baselineFeatures() []Feature {
	shipped := StatusShipped
	planned := StatusPlanned
	inProgress := StatusInProgress
	omit := StatusIntentionalOmit
	parity := ClassParity
	ext := ClassExtension

	return []Feature{
		// Foundation and cross-cutting
		row("foundation.charter", "program charter and product contract", "foundation", shipped, shipped, parity, "0001"),
		rowWithTests("foundation.features-inventory", "feature inventory and parity matrix", "foundation", omit, shipped, ext, "0002", []string{
			"internal/features",
			"features/inventory.json",
			"internal/cli",
		}),
		rowWithTests("foundation.architecture", "target Go architecture and boundaries", "foundation", shipped, shipped, parity, "0003", []string{"internal/archcheck"}),
		rowWithTests("foundation.repository-bootstrap", "repository bootstrap and quality gates", "foundation", shipped, shipped, parity, "0004", []string{"internal/testkit", "internal/bootstrap", "internal/cli"}),
		rowWithTests("foundation.error-model", "stable error codes and diagnostics", "foundation", shipped, shipped, parity, "0005", []string{"internal/apperr", "internal/diagnostics", "internal/trace", "internal/cli"}),
		rowWithTests("foundation.config-identity", "configuration and PM identity detection", "foundation", shipped, shipped, parity, "0006", []string{"internal/config", "internal/project", "internal/cli"}),
		rowWithTests("foundation.data-model", "canonical manifest and graph interfaces", "foundation", shipped, shipped, parity, "0007", []string{"internal/graph", "internal/manifest", "internal/resolver", "internal/lockfile", "internal/plan", "internal/snapshot", "internal/policy"}),
		rowWithTests("foundation.testing-strategy", "fixtures, fuzzing, and conformance harness", "foundation", shipped, shipped, parity, "0008", []string{"internal/testkit", "tests/integration", "tests/conformance"}),
		rowWithTests("foundation.release-train", "MVP dependency graph and release train", "foundation", shipped, shipped, parity, "0009", []string{"internal/releasetrain"}),
		rowWithTests("foundation.cli", "m and mx CLI foundation", "foundation", shipped, shipped, parity, "0010", []string{"internal/cli", "cmd/m", "cmd/mx", "internal/app"}),
		rowWithTests("cli.presentation-foundation", "CLI presentation contract and capability resolver", "cli", omit, shipped, ext, "0010", []string{
			"internal/presentation",
			"internal/archcheck",
			"docs/architecture/cli-presentation.md",
		}),
		rowWithTests("cli.rich-human-output", "rich human static output and design system", "cli", omit, shipped, ext, "0010", []string{
			"internal/presentation",
			"internal/cli",
		}),
		rowWithTests("cli.accessible-output", "accessible append-only output and numbered prompts", "cli", omit, shipped, ext, "0010", []string{
			"internal/presentation/prompt",
			"internal/prompt",
			"docs/accessibility.md",
			"plans/ux/accessibility-evidence.md",
		}),
		rowWithTests("cli.rich-errors", "typed ErrorView human errors", "cli", omit, shipped, ext, "0010", []string{
			"internal/presentation",
			"internal/diagnostics",
			"internal/cli",
		}),
		rowWithTests("cli.install-progress", "plain and rich install-family progress", "cli", omit, shipped, ext, "0010", []string{
			"internal/presentation",
			"internal/cli",
		}),
		rowWithTests("cli.runner-progress", "runner and workspace presentation with suspend/resume", "cli", omit, shipped, ext, "0010", []string{
			"internal/presentation",
			"internal/cli",
			"docs/runner.md",
		}),
		rowWithTests("cli.prompt-system", "prompt policy and Huh/accessible adapters", "cli", omit, shipped, ext, "0010", []string{
			"internal/prompt",
			"internal/presentation/prompt",
			"internal/lifecycle",
		}),
		rowWithTests("cli.markdown-help", "topic help Markdown and optional pager", "cli", omit, shipped, ext, "0010", []string{
			"internal/help",
			"internal/presentation/help",
			"internal/presentation/pager",
			"internal/cli",
		}),
		rowWithTests("cli.ux-certification", "CLI UX conformance matrix and platform evidence", "cli", omit, shipped, ext, "0010", []string{
			"docs/evidence/cli-ux",
			"plans/ux/performance-baseline.md",
			"plans/ux/charm-dependency-review.md",
			"tests/conformance/cli-ux",
			"m conformance run cli-ux",
		}),
		rowWithTests("foundation.manifest-discovery", "package.json and project discovery", "foundation", shipped, shipped, parity, "0011", []string{"internal/manifest", "internal/project", "internal/workspace", "internal/cli"}),
		rowWithTests("foundation.core-stabilization", "package-manager core stabilization gate", "foundation", shipped, shipped, parity, "0031", []string{
			"docs/core-certification.md",
			"docs/schema-freeze.md",
			"docs/security-pm-core.md",
			"testdata/certification/sign-off-checklist.md",
			"tests/conformance",
			"tests/integration",
			"tools/conformance/verify-fixtures",
			"tools/ci/verify-crash-shards",
		}),
		rowWithTests("foundation.runner-stabilization", "runner stabilization gate", "foundation", shipped, shipped, parity, "0046", []string{
			"internal/conformance",
			"internal/cli/conformance_runner_cmd.go",
			"tests/conformance/runner",
			"tests/conformance/runner-matrix",
			"docs/runner-compatibility.md",
		}),
		row("foundation.runtime-stabilization", "runtime stabilization gate", "foundation", shipped, planned, parity, "0057"),
		row("cross.conformance-program", "continuous conformance certification", "cross-cutting", shipped, planned, parity, "0080"),
		row("cross.performance-program", "performance measurement and gates", "cross-cutting", shipped, planned, parity, "0081"),
		row("cross.threat-model", "threat model and security reviews", "cross-cutting", shipped, planned, parity, "0082"),
		row("cross.migration-map", "Nub Rust to Mew Go migration map", "cross-cutting", shipped, planned, parity, "0083"),
		row("cross.versioning-policy", "versioning and support policy", "cross-cutting", shipped, planned, parity, "0084"),
		row("cross.dependency-roadmap", "Go dependency selection roadmap", "cross-cutting", shipped, planned, parity, "0085"),
		row("cross.ai-agent-protocol", "AI agent implementation protocol", "cross-cutting", shipped, planned, parity, "0086"),
		row("cross.definition-of-done", "global definition of done", "cross-cutting", shipped, planned, parity, "0087"),
		row("cross.reference-index", "reference index and research sources", "cross-cutting", shipped, planned, parity, "0088"),
		row("cross.research-spikes", "open research spikes and decision gates", "cross-cutting", shipped, planned, parity, "0089"),
		row("cross.future-backlog", "post-parity future extensions backlog", "cross-cutting", omit, planned, ext, "0090"),

		// Package-manager commands
		rowWithTests("pm.install", "install / i", "package-manager", shipped, shipped, parity, "0016", []string{"internal/app", "internal/cli", "tests/integration"}),
		rowWithTests("pm.ci", "ci / frozen clean install", "package-manager", shipped, shipped, parity, "0016", []string{"internal/cli", "tests/integration"}),
		rowWithTests("pm.add-remove-update", "add / remove / update", "package-manager", shipped, shipped, parity, "0016", []string{"internal/app", "internal/cli", "tests/integration"}),
		row("pm.import-dedupe-prune", "import / dedupe / prune / rebuild", "package-manager", shipped, planned, parity, "0026"),
		row("pm.list-why-outdated", "list / why / outdated / view", "package-manager", shipped, planned, parity, "0026"),
		row("pm.fetch-pack-publish", "fetch / pack / publish", "package-manager", shipped, planned, parity, "0014"),
		rowWithTests("pm.store-cache-config", "store / cache / config", "package-manager", shipped, inProgress, parity, "0012", []string{"internal/cli", "internal/config", "internal/registry"}),
		rowWithTests("pm.bench-install", "m bench install harness", "package-manager", omit, shipped, ext, "0029", []string{"internal/app", "internal/cli", "tests/integration"}),
		row("pm.global-install", "global installs where retained", "package-manager", shipped, planned, parity, "0026"),

		// Resolver
		rowWithTests("resolver.semver", "npm semver ranges and tags", "resolver", shipped, shipped, parity, "0013", []string{"internal/semver", "internal/resolver", "internal/registry"}),
		rowWithTests("resolver.transitive-graph", "transitive graph and cycles", "resolver", shipped, shipped, parity, "0013", []string{"internal/resolver", "tests/integration"}),
		rowWithTests("resolver.peers", "peer dependencies and peer contexts", "resolver", shipped, shipped, parity, "0020", []string{"internal/resolver", "fixtures/resolver/peers/react-ecosystem", "tests/integration"}),
		rowWithTests("resolver.optional-platform", "optional/dev/platform dependencies", "resolver", shipped, shipped, parity, "0020", []string{"internal/resolver", "fixtures/resolver/optional-platform", "tests/integration"}),
		rowWithTests("resolver.overrides", "overrides and resolutions", "resolver", shipped, shipped, parity, "0020", []string{"internal/resolver", "fixtures/resolver/overrides-nested", "tests/integration"}),
		rowWithTests("resolver.workspace-protocol", "workspace protocol and catalogs", "resolver", shipped, shipped, parity, "0022", []string{"internal/resolver", "fixtures/projects/workspace-protocol", "tests/integration"}),
		rowWithTests("resolver.aliases", "aliases and npm protocol", "resolver", shipped, shipped, parity, "0020", []string{"internal/resolver", "tests/integration"}),
		row("resolver.git-sources", "Git, hosted Git, file, link, portal, tarball", "resolver", shipped, planned, parity, "0027"),
		row("resolver.patches", "patch dependencies", "resolver", shipped, planned, parity, "0027"),
		rowWithTests("resolver.explain", "explain version selection and peer conflicts", "resolver", shipped, shipped, ext, "0028", []string{"internal/resolver", "internal/cli", "tests/integration", "fixtures/explain"}),
		rowWithTests("plan.preview", "install mutation plan preview", "plan", omit, shipped, ext, "0028", []string{"internal/plan", "internal/app", "internal/cli", "tests/integration"}),
		rowWithTests("resolver.minimum-release-age", "minimum release age", "resolver", shipped, shipped, parity, "0013", []string{"internal/resolver", "internal/policy"}),

		// Registry
		rowWithTests("registry.npm-compatible", "npm-compatible registries", "registry", shipped, shipped, parity, "0012", []string{"internal/registry", "internal/fetch", "internal/cli", "tests/integration"}),
		rowWithTests("registry.scoped-private", "scoped and private registries", "registry", shipped, shipped, parity, "0012", []string{"internal/registry", "internal/config"}),
		rowWithTests("registry.transport", "proxy, custom CA, redirects, gzip", "registry", shipped, shipped, parity, "0012", []string{"internal/fetch", "internal/registry"}),
		rowWithTests("registry.metadata-cache", "metadata and tarball cache", "registry", shipped, shipped, parity, "0012", []string{"internal/registry", "tests/integration"}),
		row("registry.integrity", "SHA-512 SRI and legacy shasum", "registry", shipped, planned, parity, "0014"),
		row("registry.safe-extraction", "safe archive extraction", "registry", shipped, planned, parity, "0014"),
		rowWithTests("registry.offline", "offline and prefer-offline", "registry", shipped, shipped, parity, "0012", []string{"internal/registry"}),

		// Lockfiles
		rowWithTests("lockfile.m-lock", "m.lock native format", "lockfile", omit, shipped, ext, "0015", []string{"internal/lockfile/mlock", "internal/cli", "tests/integration"}),
		row("lockfile.nub-lock", "nub.lock read/write", "lockfile", shipped, planned, parity, "0023"),
		row("lockfile.pnpm", "pnpm lockfiles", "lockfile", shipped, planned, parity, "0023"),
		row("lockfile.npm", "package-lock and shrinkwrap", "lockfile", shipped, planned, parity, "0024"),
		row("lockfile.bun", "bun.lock", "lockfile", shipped, planned, parity, "0025"),
		row("lockfile.yarn-classic", "Yarn Classic", "lockfile", shipped, planned, parity, "0025"),
		row("lockfile.yarn-berry", "Yarn Berry and PnP artifacts", "lockfile", shipped, planned, parity, "0025"),
		row("lockfile.preservation", "existing-format preservation", "lockfile", shipped, planned, parity, "0023"),
		row("lockfile.semantic-diff", "semantic diff and validation", "lockfile", shipped, planned, ext, "0028"),
		row("lockfile.migration", "explicit lock migration and loss report", "lockfile", shipped, planned, ext, "0028"),

		// Linker and reliability
		rowWithTests("linker.hoisted", "hoisted node_modules", "linker", shipped, shipped, parity, "0016", []string{"internal/linker/hoisted", "tests/integration"}),
		row("linker.isolated", "isolated virtual store", "linker", shipped, shipped, parity, "0019"),
		row("linker.global-store", "global content-addressed store", "linker", shipped, shipped, parity, "0018"),
		rowWithTests("linker.platform-links", "hardlink / symlink / junction behavior", "linker", shipped, shipped, parity, "0018", []string{"internal/linker/planner", "tests/integration"}),
		rowWithTests("linker.reflink-planner", "reflink and automatic filesystem planning", "linker", omit, shipped, ext, "0018", []string{"internal/linker/planner"}),
		rowWithTests("linker.transactional-install", "transactional install and recovery", "linker", shipped, shipped, ext, "0017", []string{"internal/transaction", "tests/integration"}),
		rowWithTests("linker.rollback-history", "instant rollback and history", "linker", omit, shipped, ext, "0017", []string{"internal/snapshot", "tests/integration"}),
		row("linker.time-travel", "dependency time travel", "linker", omit, planned, ext, "0028"),
		row("linker.capsules", "portable capsules", "linker", omit, planned, ext, "0029"),

		// Lifecycle and security
		row("lifecycle.scripts", "lifecycle scripts", "lifecycle", shipped, shipped, parity, "0021"),
		row("lifecycle.trusted-deps", "trusted dependencies / build approval", "lifecycle", shipped, shipped, parity, "0021"),
		row("lifecycle.sandbox", "script sandbox", "lifecycle", shipped, shipped, parity, "0021"),
		row("lifecycle.build-cache", "build-output cache", "lifecycle", shipped, shipped, parity, "0021"),
		row("security.audit", "audit and advisories", "security", shipped, planned, parity, "0030"),
		rowWithTests("security.sbom", "SBOM export", "security", shipped, shipped, parity, "0030", []string{
			"internal/sbom/sbom_test.go",
			"internal/app/sbom_test.go",
			"tests/integration/sbom_test.go",
		}),
		rowWithTests("security.provenance", "provenance and signatures", "security", shipped, shipped, parity, "0030", []string{
			"internal/provenance/verify_test.go",
			"tests/integration/provenance_test.go",
		}),
		row("security.policy-as-code", "policy-as-code", "security", shipped, planned, ext, "0030"),

		// Workspaces and scripts
		row("workspace.discovery", "workspace discovery and graph", "workspace", shipped, planned, parity, "0022"),
		row("workspace.filtered-commands", "recursive and filtered commands", "workspace", shipped, planned, parity, "0022"),
		row("workspace.parallel-scripts", "topological and parallel script execution", "workspace", shipped, planned, parity, "0041"),
		rowWithTests("runner.m-run", "m run script", "runner", shipped, shipped, parity, "0040", []string{
			"internal/runner",
			"internal/cli/run_cmd.go",
			"tests/conformance/runner",
		}),
		rowWithTests("runner.hooks-env", "pre/post hooks and npm environment", "runner", shipped, shipped, parity, "0040", []string{
			"internal/runner/env.go",
			"internal/runner/hooks.go",
		}),
		rowWithTests("runner.reporters", "reporters and NDJSON", "runner", shipped, shipped, parity, "0005", []string{
			"internal/diagnostics",
		}),
		rowWithTests("runner.direct-shortcuts", "direct m dev / m start shortcuts", "runner", omit, shipped, ext, "0042", []string{
			"internal/cli/dispatch_test.go",
			"tests/conformance/runner/direct_dispatch_gates_test.go",
		}),
		row("runner.interactive-select", "interactive script selection", "runner", omit, planned, ext, "0090"),

		// Executable runner
		row("exec.local", "local package binary execution (m exec)", "executable", shipped, planned, parity, "0043"),
		row("exec.mx-dlx", "local-first temporary execution (mx)", "executable", shipped, planned, parity, "0044"),
		row("exec.package-flags", "package flags and multiple packages", "executable", shipped, planned, parity, "0044"),
		row("exec.consent", "consent and non-TTY fail-closed behavior", "executable", shipped, planned, parity, "0044"),
		row("exec.cache-offline", "execution cache and offline mode", "executable", shipped, planned, parity, "0044"),
		row("exec.snapshot-capsule", "snapshot and capsule execution", "executable", omit, planned, ext, "0045"),

		// Runtime
		row("runtime.js-cjs-esm", "run JS/CJS/ESM", "runtime", shipped, shipped, parity, "0050"),
		row("runtime.typescript", "run TS/MTS/CTS", "runtime", shipped, shipped, parity, "0051"),
		row("runtime.jsx", "JSX/TSX", "runtime", shipped, shipped, parity, "0052"),
		row("runtime.decorators", "legacy and standard decorators", "runtime", shipped, shipped, parity, "0052"),
		row("runtime.decorator-metadata", "decorator metadata", "runtime", shipped, shipped, parity, "0052"),
		row("runtime.tsconfig-paths", "tsconfig and path aliases", "runtime", shipped, shipped, parity, "0051"),
		row("runtime.sourcemaps", "source maps", "runtime", shipped, shipped, parity, "0052"),
		row("runtime.loaders", "custom loaders and preloads", "runtime", shipped, shipped, parity, "0053"),
		row("runtime.env-loading", "environment auto-loading", "runtime", shipped, shipped, parity, "0054"),
		row("runtime.workers-storage", "workers and Web Storage", "runtime", shipped, shipped, parity, "0054"),
		row("runtime.watch", "watch and automatic restart", "runtime", shipped, shipped, parity, "0055"),
		row("runtime.debugger", "debugger and inspector", "runtime", shipped, shipped, parity, "0056"),
		row("runtime.plain-node", "plain Node escape hatch", "runtime", shipped, shipped, parity, "0050"),

		// Node, PM, shims
		row("node.install-use", "Node install/use/list/remove", "node-manager", shipped, planned, parity, "0060"),
		row("node.version-discovery", "nvmrc/node-version/engines discovery", "node-manager", shipped, planned, parity, "0060"),
		row("node.auto-provision", "automatic Node provisioning", "node-manager", shipped, planned, parity, "0060"),
		row("pm-manager.detect-pin", "PM detect/pin/update/migrate/cache", "pm-manager", shipped, planned, parity, "0061"),
		row("shim.corepack", "Corepack-style shims", "shim", shipped, planned, parity, "0062"),
		row("shim.node-plain", "Node PATH shim without augmentation", "shim", shipped, planned, parity, "0062"),
		row("shim.mew-self", "Mew self-version shim", "shim", shipped, planned, parity, "0062"),

		// Project, plugins, distribution
		row("product.ts-init", "TypeScript-first init", "product", shipped, planned, parity, "0070"),
		row("product.template-delegation", "template delegation through mx", "product", shipped, planned, parity, "0070"),
		row("plugin.external-verbs", "external verb plugins (m-<verb>)", "plugin", shipped, planned, parity, "0071"),
		row("dist.installers", "direct installers", "distribution", shipped, planned, parity, "0072"),
		row("dist.channels", "package channels", "distribution", shipped, planned, parity, "0072"),
		row("dist.self-update", "self-update", "distribution", shipped, planned, parity, "0072"),
		row("dist.github-action", "GitHub Action", "distribution", shipped, planned, parity, "0073"),
		row("dist.docker-builders", "Docker and hosted builders", "distribution", shipped, planned, parity, "0074"),
	}
}
