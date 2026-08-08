package runtime

import "io"

// AugmentationMode controls how the Node process is augmented.
type AugmentationMode string

const (
	// AugmentDefault uses the configured augmentation level.
	AugmentDefault AugmentationMode = ""
	// AugmentNone runs plain stock Node with no injection.
	AugmentNone AugmentationMode = "none"
	// AugmentFull injects all configured preloads.
	AugmentFull AugmentationMode = "full"
)

// LaunchContribution adds runtime inputs from an app-level orchestrator
// without importing transform types into the runtime package.
type LaunchContribution struct {
	ExtraEnv      []string
	ExtraPreloads []PreloadAsset
	CleanupHook   func() error // called after Node exits (on any path)
}

// LaunchRequest describes a Node launch from the CLI.
type LaunchRequest struct {
	Entrypoint        string
	AppArgs           []string
	NodeV8Args        []string
	WorkingDir        string
	EnvOverlay        []string
	AugmentationMode  AugmentationMode
	Stdio             LaunchStdio
	ExperimentalState map[string]string
	Contribution      *LaunchContribution
	// Loaders are user-specified ESM loader paths (--loader flag).
	// They are registered via module.register() by credential-grabber.cjs
	// (augmented mode) or loader-register.mjs (--node mode) through the
	// MEW_USER_LOADERS env var. No longer injected as bare --import.
	Loaders []string
}

// LaunchStdio configures child process I/O.
type LaunchStdio struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// LaunchPlan is a fully resolved Node launch.
type LaunchPlan struct {
	NodeExe           string
	NodeVersion       string
	NodeCapabilities  []string
	NodeArgv          []string
	CredentialPreload *PreloadAsset
	CustomLoaders     []PreloadAsset // user --loader file:// URLs for env passthrough (no longer injected as --import)
	LoaderShimPath    string         // loader-register.mjs path for --node mode with loaders
	PreloadAssets     []PreloadAsset
	Entrypoint        string
	AppArgs           []string
	EnvChanges        []string
	ZeroAugmentation  bool
	EnableSourceMaps  bool             // add --enable-source-maps to Node when capability present
	Inspector         *InspectorConfig // normalized inspector flags (nil when not requested)
	CleanupHook       func() error
}

// PreloadAsset describes a resolved preload path for Node argv.
type PreloadAsset struct {
	Path       string
	ModuleType string // "cjs" or "esm"
}

// RuntimeAsset describes an embedded runtime file.
type RuntimeAsset struct {
	LogicalName    string
	EmbeddedPath   string
	ExtractionPath string
	ModuleType     string
	Size           int64
	SHA256         string
	BundleVersion  string
}
