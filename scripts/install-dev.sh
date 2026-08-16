#!/usr/bin/env bash
# MewJS development installer (Unix). LF line endings required.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=lib/devinstall.sh
source "$SCRIPT_DIR/lib/devinstall.sh"

stage() { printf '[%s] %s\n' "$1" "$2"; }

BUILD_ONLY=0
SKIP_PATH=0
SKIP_COMPLETION=0
SKIP_VERIFY=0
INSTALL_DIR=''
FLAG_GOOS=''
FLAG_GOARCH=''
FLAG_VERSION=''
FORCE=0

usage() {
  cat <<'EOF'
Usage: install-dev.sh [options]

  --build-only          Build bin/m and bin/mx only; skip install
  --skip-path           Skip PATH profile updates
  --skip-completion     Skip shell completion generation
  --skip-verify         Skip post-install verification
  --install-dir <dir>   Custom install directory
  --goos <os>           Target GOOS (windows|linux|darwin)
  --goarch <arch>       Target GOARCH (amd64|arm64)
  --version <ver>       Override build version metadata
  --force               Replace conflicting files
  -h, --help            Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --build-only) BUILD_ONLY=1; shift ;;
    --skip-path) SKIP_PATH=1; shift ;;
    --skip-completion) SKIP_COMPLETION=1; shift ;;
    --skip-verify) SKIP_VERIFY=1; shift ;;
    --install-dir) INSTALL_DIR="$2"; shift 2 ;;
    --goos) FLAG_GOOS="$2"; shift 2 ;;
    --goarch) FLAG_GOARCH="$2"; shift 2 ;;
    --version) FLAG_VERSION="$2"; shift 2 ;;
    --force) FORCE=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) devinstall_die "unknown argument: $1" ;;
  esac
done

DEVINSTALL_REPO_ROOT="$REPO_ROOT"
cd "$DEVINSTALL_REPO_ROOT"

stage detect 'detecting host and target'
devinstall_detect_host
devinstall_resolve_target "$FLAG_GOOS" "$FLAG_GOARCH"
EMULATED=0
if devinstall_is_emulated; then EMULATED=1; fi
CAN_INSTALL="$DEVINSTALL_CAN_INSTALL"
stage detect "host ${DEVINSTALL_HOST_GOOS}/${DEVINSTALL_HOST_GOARCH}"
stage detect "target ${DEVINSTALL_TARGET_GOOS}/${DEVINSTALL_TARGET_GOARCH}"
if [[ "$EMULATED" == 1 ]]; then
  stage detect 'warning: target arch differs from host (emulation/cross)'
fi

stage check 'validating prerequisites'
devinstall_check_go_version "$(devinstall_go_mod_version "$REPO_ROOT")" >/dev/null
devinstall_check_repo
devinstall_resolve_metadata "$FLAG_VERSION"
DIRTY_LABEL=''
if [[ "$DEVINSTALL_DIRTY" == 1 ]]; then DIRTY_LABEL='+dirty'; fi
stage check "version=${DEVINSTALL_VERSION}${DIRTY_LABEL} commit=${DEVINSTALL_SHORT_COMMIT} target=${DEVINSTALL_TARGET_GOOS}/${DEVINSTALL_TARGET_GOARCH}"

devinstall_build

if [[ "$BUILD_ONLY" == 1 ]]; then
  stage done 'build-only complete'
  DEVINSTALL_INSTALL_DIR=''
  DEVINSTALL_COMPLETION_BASE=''
  devinstall_print_summary
  exit 0
fi

if [[ "$CAN_INSTALL" != 1 ]]; then
  devinstall_die 'cannot install cross-compiled binaries on this host (use --build-only)'
fi

if [[ -z "$INSTALL_DIR" ]]; then
  DEVINSTALL_INSTALL_DIR="$(devinstall_default_install_dir_unix)"
else
  DEVINSTALL_INSTALL_DIR="$INSTALL_DIR"
fi
DEVINSTALL_COMPLETION_BASE="$(devinstall_completion_base_from_install_dir "$DEVINSTALL_INSTALL_DIR")"

stage install "installing to $DEVINSTALL_INSTALL_DIR"
devinstall_install_unix_files "$DEVINSTALL_INSTALL_DIR" "$FORCE"

if [[ "$SKIP_PATH" != 1 ]]; then
  stage path 'updating shell profile PATH'
  devinstall_path_unix "$DEVINSTALL_INSTALL_DIR"
else
  stage path 'skipped (--skip-path)'
fi

if [[ "$SKIP_COMPLETION" != 1 ]]; then
  stage completion "generating completions in ${DEVINSTALL_COMPLETION_BASE}"
  devinstall_completion_unix "$DEVINSTALL_INSTALL_DIR" "$DEVINSTALL_COMPLETION_BASE"
else
  stage completion 'skipped (--skip-completion)'
  DEVINSTALL_COMPLETION_BASE=''
fi

if [[ "$SKIP_VERIFY" != 1 ]]; then
  stage verify 'running verification'
  devinstall_verify_unix "$DEVINSTALL_INSTALL_DIR" "$DEVINSTALL_COMPLETION_BASE"
else
  stage verify 'skipped (--skip-verify)'
fi

stage done 'development install complete'
devinstall_print_summary
