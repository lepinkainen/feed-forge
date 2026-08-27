#!/usr/bin/env bash
#
# Deploy build/feed-forge-linux to the remote server, atomically, and roll the
# systemd user service onto the new binary.
#
# Flow: resolve the final remote path, upload to <path>.new, verify the
# uploaded binary runs and reports the expected version, then rename it over
# the live binary. The rename is atomic on the remote filesystem, so nothing
# ever executes a half-written binary, and a stale process holding the old
# file open cannot fail the deploy (the upload creates a fresh inode).
#
# When configs/systemd/feed-forge.service exists locally and the remote has a
# systemd user manager, the deploy then installs the unit, validates the live
# configuration with the NEW binary, and restarts the service. A running
# daemon keeps the old inode until that restart, so a failed validation
# leaves the previous version serving. Without systemd (or before the first
# cutover) the deploy stops after the rename, as before.
#
# Usage:
#   scripts/publish.sh EXPECTED_VERSION
#
# Environment:
#   PUBLISH_DESTINATION  scp-style destination, e.g. user@host:bin/ or
#                        user@host:bin/feed-forge. A directory destination
#                        deploys as "feed-forge", the name the systemd unit
#                        and any remaining cron jobs run. A file destination
#                        is used verbatim.
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

unit="configs/systemd/feed-forge.service"
if [ ! -f "$unit" ]; then
  echo "no $unit in the repo; binary-only deploy done"
  exit 0
fi

if ! ssh "$host" 'command -v systemctl >/dev/null 2>&1'; then
  echo "remote has no systemctl; binary-only deploy done"
  exit 0
fi

ssh "$host" 'mkdir -p .config/systemd/user'
scp "$unit" "$host:.config/systemd/user/feed-forge.service"

# Validate the live configuration with the new binary before restarting: on
# failure the running daemon keeps serving the old inode. Non-interactive ssh
# may lack the session-bus environment, so set it explicitly; lingering is
# enabled, so the user manager is running.
# shellcheck disable=SC2087 # client-side expansion of $bin is intentional
ssh "$host" sh <<EOF
set -e
export XDG_RUNTIME_DIR="/run/user/\$(id -u)"
export DBUS_SESSION_BUS_ADDRESS="unix:path=\$XDG_RUNTIME_DIR/bus"

if ! '$bin' validate-config; then
  echo "configuration invalid for the new binary; NOT restarting feed-forge" >&2
  exit 1
fi

systemctl --user daemon-reload
systemctl --user enable feed-forge.service >/dev/null 2>&1 || true
systemctl --user restart feed-forge.service

if ! systemctl --user is-active --quiet feed-forge.service; then
  echo "feed-forge.service is not active after restart:" >&2
  journalctl --user -u feed-forge -n 20 --no-pager >&2 || true
  exit 1
fi
echo "feed-forge.service restarted and active"
EOF
