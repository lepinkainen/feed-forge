#!/usr/bin/env bash
#
# One-time migration for the opengraph -> linkpreview rename.
#
# feed-forge replaced the OpenGraph metadata cache (opengraph.db) with the
# trafilatura-backed link-preview cache (linkpreview.db). The old database is a
# disposable cache (24h TTL) whose schema no longer matches the new code, so the
# migration is simply: delete the orphaned opengraph.db (and its SQLite sidecar
# files). The new binary recreates linkpreview.db on its next run and repopulates
# it from the source feeds.
#
# Usage:
#   scripts/migrate-opengraph-to-linkpreview.sh [CACHE_DIR]
#
# CACHE_DIR is optional. If omitted, the same resolution the binary uses applies:
#   1. $XDG_CACHE_HOME/feed-forge   (if XDG_CACHE_HOME is set)
#   2. $HOME/.cache/feed-forge
# If you run feed-forge with --cache-dir, pass that same directory here.
#
# Set DRY_RUN=1 to print what would be removed without deleting anything.

set -euo pipefail

resolve_cache_dir() {
	if [ "$#" -ge 1 ] && [ -n "${1:-}" ]; then
		printf '%s\n' "$1"
		return
	fi
	if [ -n "${XDG_CACHE_HOME:-}" ]; then
		printf '%s\n' "$XDG_CACHE_HOME/feed-forge"
		return
	fi
	printf '%s\n' "$HOME/.cache/feed-forge"
}

CACHE_DIR="$(resolve_cache_dir "${1:-}")"
echo "Cache directory: $CACHE_DIR"

if [ ! -d "$CACHE_DIR" ]; then
	echo "Cache directory does not exist; nothing to migrate."
	exit 0
fi

removed=0
for f in opengraph.db opengraph.db-wal opengraph.db-shm; do
	path="$CACHE_DIR/$f"
	if [ -e "$path" ]; then
		if [ "${DRY_RUN:-0}" = "1" ]; then
			echo "[dry-run] would remove: $path"
		else
			rm -f "$path"
			echo "removed: $path"
		fi
		removed=$((removed + 1))
	fi
done

if [ "$removed" -eq 0 ]; then
	echo "No opengraph.db files found; already migrated or fresh install."
else
	echo "Done. The new linkpreview.db will be created on the next feed-forge run."
fi
