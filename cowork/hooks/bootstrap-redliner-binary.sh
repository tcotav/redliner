#!/usr/bin/env bash
# Downloads and installs the redliner Go binary for the current
# platform, from the GitHub Release matching this plugin's own
# .claude-plugin/plugin.json version -- the replacement for committing
# ~9MB binaries to the tracked tree (see TODO.md's "Port to a compiled
# language for distributable binaries?" section, Phase 6's "Superseded"
# note). Called from a SessionStart hook, which is the one place a
# plugin actually gets ${CLAUDE_PLUGIN_ROOT}/${CLAUDE_PLUGIN_DATA}
# substituted into a command -- confirmed via Claude Code's own docs
# (env vars are exported to hook/MCP/LSP subprocesses only, not to bare
# bin/ invocations) and via a live write-test before this script was
# written, not assumed.
#
# Usage: bootstrap-redliner-binary.sh <dest-path>
#   The Cowork plugin's hook passes ${CLAUDE_PLUGIN_DATA}/bin/redliner,
#   which cowork/.claude-plugin/plugin.json's mcpServers.command then
#   references directly -- that field supports the same substitution a
#   bin/ invocation doesn't get, so no on-disk indirection is needed
#   there.
#
# NOTE: hooks/bootstrap-redliner-binary.sh (the CLI plugin's copy) is
# the same script, identical apart from this header comment --
# deliberately duplicated rather than symlinked. This file *was* a
# symlink to that one, which silently dangled the moment that one was
# briefly deleted, stopping Cowork's binary from downloading at all (see
# TODO.md, 2026-08-12). Both plugins download their binary through their
# own copy again; two 4KB files is the cheaper failure mode. Keep the
# executable part in sync.
set -euo pipefail

DEST="${1:?usage: bootstrap-redliner-binary.sh <dest-path>}"
REPO="tcotav/redliner"
PLUGIN_JSON="${CLAUDE_PLUGIN_ROOT}/.claude-plugin/plugin.json"
VERSION_MARKER="${DEST}.version"

# Read the version from this plugin's own manifest rather than
# hardcoding it a third time (it already lives in two plugin.json files)
# -- this project has been bitten before by a name/value living in more
# places than expected (the edaitor->redliner rename).
VERSION="$(grep -o '"version"[[:space:]]*:[[:space:]]*"[^"]*"' "$PLUGIN_JSON" | head -1 | sed -E 's/.*"([^"]+)"$/\1/')"
if [ -z "$VERSION" ]; then
  echo "redliner bootstrap: could not read a version from $PLUGIN_JSON" >&2
  exit 1
fi

# Already have this exact version -- nothing to do. Re-downloads
# whenever the plugin's own version bumps, the same diff-based reinstall
# idea the old venv hook used for requirements.txt (see git history for
# cowork/hooks/hooks.json before this replaced it).
if [ -x "$DEST" ] && [ "$(cat "$VERSION_MARKER" 2>/dev/null || true)" = "$VERSION" ]; then
  exit 0
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
RAW_ARCH="$(uname -m)"
case "$RAW_ARCH" in
  arm64|aarch64) ARCH=arm64 ;;
  x86_64|amd64)  ARCH=amd64 ;;
  *)
    echo "redliner bootstrap: unsupported architecture '$RAW_ARCH'. Releases cover darwin and linux on amd64/arm64; the CLI/MCP tools will be unavailable this session. Add a (GOOS, GOARCH) pair to the release workflow's PLATFORMS list to cover it." >&2
    exit 1
    ;;
esac

case "$OS" in
  darwin|linux) ;;
  *)
    echo "redliner bootstrap: unsupported OS '$OS' -- releases cover darwin and linux only. Windows needs both an asset naming fix and a .exe build; see the release workflow's header comment." >&2
    exit 1
    ;;
esac

ASSET="redliner-${VERSION}-${OS}-${ARCH}"
BASE_URL="https://github.com/${REPO}/releases/download/v${VERSION}"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

if ! curl -fsSL "${BASE_URL}/${ASSET}" -o "${WORKDIR}/${ASSET}"; then
  echo "redliner bootstrap: failed to download ${BASE_URL}/${ASSET} -- no release built for this version/platform yet?" >&2
  exit 1
fi
if ! curl -fsSL "${BASE_URL}/checksums.txt" -o "${WORKDIR}/checksums.txt"; then
  echo "redliner bootstrap: failed to download ${BASE_URL}/checksums.txt" >&2
  exit 1
fi

EXPECTED="$(grep " ${ASSET}\$" "${WORKDIR}/checksums.txt" | awk '{print $1}')"
if [ -z "$EXPECTED" ]; then
  echo "redliner bootstrap: no checksum entry for $ASSET in checksums.txt -- refusing to install an unverified binary" >&2
  exit 1
fi
if command -v shasum >/dev/null 2>&1; then
  ACTUAL="$(shasum -a 256 "${WORKDIR}/${ASSET}" | awk '{print $1}')"
elif command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "${WORKDIR}/${ASSET}" | awk '{print $1}')"
else
  echo "redliner bootstrap: neither shasum nor sha256sum found -- refusing to install an unverified binary" >&2
  exit 1
fi
if [ "$ACTUAL" != "$EXPECTED" ]; then
  echo "redliner bootstrap: checksum mismatch for $ASSET (expected $EXPECTED, got $ACTUAL) -- refusing to install" >&2
  exit 1
fi

mkdir -p "$(dirname "$DEST")"
mv "${WORKDIR}/${ASSET}" "$DEST"
chmod +x "$DEST"
echo "$VERSION" > "$VERSION_MARKER"
