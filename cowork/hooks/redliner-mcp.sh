#!/usr/bin/env bash
# Launches the redliner MCP server, downloading the binary first if it
# isn't there yet.
#
# Why this exists. The Cowork plugin's binary is fetched by a
# SessionStart hook into ${CLAUDE_PLUGIN_DATA}/bin/redliner, and
# plugin.json used to point mcpServers.command straight at that path. On
# a fresh install the MCP spawn beats the hook, so the very first session
# after installing got:
#
#   plugin:redliner-cowork:redliner (ENOENT): no such file or directory,
#   posix_spawn '.../data/redliner-cowork-redliner/bin/redliner'
#
# ...and the server stayed dead for that entire session. It worked from
# the next session on, and `claude mcp list` (a separate process, run
# later) always looked healthy -- which is why this went unnoticed
# through v0.7.0. Found 2026-09-04 by the v0.7.0 install gate.
#
# This script is committed *inside the plugin*, so it exists the instant
# the plugin is cloned into the cache. Pointing `command` at it means the
# path being spawned is always present, and the download happens inside
# the process the server is starting in rather than racing it.
#
# Nothing here may write to stdout: that is the JSON-RPC channel. The
# bootstrap script's output is redirected to stderr below for that
# reason, and every diagnostic in this file goes to >&2.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Derive the plugin root from this script's own location rather than
# trusting ${CLAUDE_PLUGIN_ROOT} to be exported into an MCP subprocess.
# It appears to be, but this file is the fix for an assumption about
# process ordering that turned out to be wrong, so it does not lean on a
# second unverified assumption to do its job. bootstrap-redliner-binary.sh
# reads CLAUDE_PLUGIN_ROOT to find plugin.json, so export the derived
# value when the environment didn't supply one.
PLUGIN_ROOT="$(cd "$HERE/.." && pwd)"
export CLAUDE_PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$PLUGIN_ROOT}"

# A ${VAR} the harness didn't substitute arrives as that literal text. It
# would otherwise be used as a path, producing a baffling error about a
# directory named '${CLAUDE_PLUGIN_DATA}'. Treat it as absent instead.
expanded() {
  case "${1:-}" in
    ""|*'${'*) return 1 ;;
    *) return 0 ;;
  esac
}

# Where the binary goes, most-explicit first. REDLINER_BIN is what
# plugin.json sets; CLAUDE_PLUGIN_DATA is the harness's own variable if
# it reaches us; the plugin root is the last resort, and works because
# the restriction on a top-level bin/ applies to *published plugin
# content*, not to a directory created at runtime.
DEST=""
for candidate in \
  "${REDLINER_BIN:-}" \
  "$([ -n "${CLAUDE_PLUGIN_DATA:-}" ] && printf '%s/bin/redliner' "$CLAUDE_PLUGIN_DATA" || true)" \
  "$PLUGIN_ROOT/bin/redliner"
do
  if expanded "$candidate"; then
    DEST="$candidate"
    break
  fi
done

if [ -z "$DEST" ]; then
  echo "redliner mcp: could not determine where to install the binary (REDLINER_BIN and CLAUDE_PLUGIN_DATA both unusable)" >&2
  exit 1
fi

# Idempotent and cheap when the binary is already current: the bootstrap
# script compares DEST.version against plugin.json and returns
# immediately. Only a first install or a version bump pays for a
# download.
if ! "$HERE/bootstrap-redliner-binary.sh" "$DEST" >&2; then
  if [ -x "$DEST" ]; then
    # A stale server beats no server -- an offline session keeps working
    # with whatever was installed last, which is the behaviour someone
    # on a plane needs. The stderr note is what makes the version
    # discrepancy findable if the tools then misbehave.
    echo "redliner mcp: bootstrap failed; starting the existing binary at $DEST, which may be an older version" >&2
  else
    echo "redliner mcp: bootstrap failed and no binary is installed at $DEST -- the redliner tools are unavailable this session" >&2
    exit 1
  fi
fi

exec "$DEST" "$@"
