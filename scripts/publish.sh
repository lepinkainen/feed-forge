#!/usr/bin/env bash
#
# Deploy build/feed-forge-linux to the remote server, atomically.
#
# Flow: resolve the final remote path, upload to <path>.new, verify the
# uploaded binary runs and reports the expected version, then rename it over
# the live binary. The rename is atomic on the remote filesystem, so cron
# never executes a half-written binary, and a stale process holding the old
# file open cannot fail the deploy (the upload creates a fresh inode).
#
# Usage:
#   scripts/publish.sh EXPECTED_VERSION
#
# Environment:
#   PUBLISH_DESTINATION  scp-style destination, e.g. user@host:bin/ or
#                        user@host:bin/feed-forge. A directory destination
#                        deploys as "feed-forge", the name the remote cron
#                        jobs run. A file destination is used verbatim.
#
# EXPECTED_VERSION must match the ldflags-injected version the uploaded
# binary prints for --version (task passes {{.VERSION}}).

set -euo pipefail

version="${1:?usage: publish.sh EXPECTED_VERSION}"
: "${PUBLISH_DESTINATION:?PUBLISH_DESTINATION is not set}"

host="${PUBLISH_DESTINATION%%:*}"
dest="${PUBLISH_DESTINATION#*:}"

# Resolve the destination to an absolute remote path: expand a leading ~,
# anchor a relative path at $HOME (scp's default base), and drop any trailing
# slash. Pipe the probe to a remote POSIX sh over stdin so it runs regardless
# of the remote login shell (fish rejects sh assignment/case syntax). The
# local $dest is expanded here; remote expansions are escaped to survive to sh.
# shellcheck disable=SC2087 # client-side expansion of $dest is intentional
bin=$(ssh "$host" sh <<EOF
d='$dest'
case "\$d" in
  "~") d="\$HOME" ;;
  "~/"*) d="\$HOME/\${d#\~/}" ;;
  /*) ;;
  *) d="\$HOME/\$d" ;;
esac
d="\${d%/}"
test -d "\$d" && d="\$d/feed-forge"
echo "\$d"
EOF
)

if [ -z "$bin" ]; then
  echo "failed to resolve remote destination from PUBLISH_DESTINATION" >&2
  exit 1
fi

scp build/feed-forge-linux "$host:$bin.new"

# shellcheck disable=SC2087 # client-side expansion of $bin/$version is intentional
ssh "$host" sh <<EOF
chmod +x '$bin.new' \
  && actual=\$('$bin.new' --version) \
  && test "\$actual" = '$version' \
  && mv -f '$bin.new' '$bin' \
  && echo "deployed \$actual to $bin" \
  || { rm -f '$bin.new'; echo "publish failed (got '\$actual', want '$version')" >&2; exit 1; }
EOF
