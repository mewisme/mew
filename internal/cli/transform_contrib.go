package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/mewisme/mew/internal/apperr"
	"github.com/mewisme/mew/internal/config"
	"github.com/mewisme/mew/internal/runtime"
	"github.com/mewisme/mew/internal/transform"
)

// buildTransformContribution creates a transform session and returns
// a LaunchContribution with the service endpoint, token, loader preload,
// and cleanup hook.
//
// Setup order:
//  1. Discover and load tsconfig.
//  2. Normalize and validate transform options.
//  3. Serialize options and compute the chain digest.
//  4. Create and start the transform session.
//  5. Return the launch contribution.
//
// Tsconfig failures are fail-closed: if a tsconfig is discovered but invalid
// or unreadable, the function returns an error. Only when discovery cleanly
// finds no config file are default options used.
func buildTransformContribution(ctx context.Context, cwd, entrypoint string, eff *config.Effective) (*runtime.LaunchContribution, error) {
	entryDir := filepath.Dir(entrypoint)

	// Step 1: Discover and load tsconfig.
	configPath, err := transform.DiscoverTsconfig(entryDir)
	if err != nil {
		return nil, wrapTsconfigErr(err, entrypoint)
	}
	var opts transform.NormalizedOptions
	var optsDigest string
	if configPath != "" {
		chain, err := transform.LoadTsconfigChain(configPath)
		if err != nil {
			return nil, wrapTsconfigErr(err, entrypoint)
		}
		if len(chain) == 0 {
			return nil, apperr.New(apperr.TransformConfigParse, "cli.transform", entrypoint,
				"tsconfig chain is empty after loading")
		}

		// Step 2: Normalize and validate transform options.
		opts, err = transform.NormalizeOptions(chain)
		if err != nil {
			return nil, wrapTsconfigErr(err, entrypoint)
		}
	}

	// Step 3a: Compute options digest from canonical JSON.
	optsDigest = opts.Digest()

	// Step 3b: Serialize options to JSON (even default zero-value options).
	optsJSON, err := json.Marshal(opts)
	if err != nil {
		return nil, apperr.Wrap(apperr.Internal, "cli.transform", entrypoint,
			fmt.Errorf("serializing transform options: %w", err))
	}

	// Step 4: Create and start the transform session.
	cacheDir := transform.TransformCacheDir(eff)
	sess, err := transform.NewSession(transform.ServiceOptions{
		Engine:   transform.NewEsbuildEngine(),
		CacheDir: cacheDir,
		Workers:  4,
		Context:  ctx,
	})
	if err != nil {
		return nil, apperr.Wrap(apperr.Internal, "cli.transform", entrypoint,
			fmt.Errorf("starting transform service: %w", err))
	}

	if err := sess.Start(); err != nil {
		_ = sess.Close()
		return nil, apperr.Wrap(apperr.RuntimeNodeStart, "cli.transform", entrypoint,
			fmt.Errorf("transform service health check: %w", err))
	}

	// Step 5: Return the launch contribution.
	extraEnv := sess.EnvOverlay()
	configDir := ""
	if configPath != "" {
		configDir = filepath.Dir(configPath)
	}
	extraEnv = append(extraEnv,
		"MEW_TRANSFORM_OPTIONS="+string(optsJSON),
		"MEW_TRANSFORM_OPTS_DIGEST="+optsDigest,
		"MEW_TRANSFORM_CONFIG_DIR="+configDir,
	)

	return &runtime.LaunchContribution{
		ExtraEnv:    extraEnv,
		CleanupHook: func() error { return sess.Close() },
	}, nil
}

// wrapTsconfigErr maps a transform.ConfigError to the appropriate apperr code.
func wrapTsconfigErr(err error, subject string) error {
	var cfgErr *transform.ConfigError
	if !asConfigError(err, &cfgErr) {
		return apperr.Wrap(apperr.Internal, "cli.transform", subject, err)
	}
	code := configErrToCode(cfgErr.Kind)
	return apperr.Wrap(code, "cli.transform", subject, err)
}

// asConfigError reports whether err is or wraps a ConfigError.
func asConfigError(err error, target **transform.ConfigError) bool {
	if err == nil {
		return false
	}
	if ce, ok := err.(*transform.ConfigError); ok {
		*target = ce
		return true
	}
	if u, ok := err.(interface{ Unwrap() error }); ok {
		return asConfigError(u.Unwrap(), target)
	}
	return false
}

// configErrToCode maps ConfigErrorKind to an apperr.Code.
func configErrToCode(kind transform.ConfigErrorKind) apperr.Code {
	switch kind {
	case transform.ConfigErrIO:
		return apperr.IO
	case transform.ConfigErrParse:
		return apperr.TransformConfigParse
	case transform.ConfigErrExtendsMissing,
		transform.ConfigErrExtendsCycle,
		transform.ConfigErrExtendsDepth,
		transform.ConfigErrExtendsPackage,
		transform.ConfigErrExtendsInvalid:
		return apperr.TransformConfigExtends
	case transform.ConfigErrOptionInvalid:
		return apperr.TransformConfigOption
	case transform.ConfigErrOptionUnsupported:
		return apperr.TransformUnsupported
	default:
		return apperr.Internal
	}
}
