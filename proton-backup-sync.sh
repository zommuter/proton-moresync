#!/bin/sh
# Run the proton-moresync backup, commit the tree, optionally push.
# Safe to invoke concurrently and from non-interactive contexts.
#
# One-time setup: see docs/proton-backup-runbook.md § Scheduling.
# Install: run `make install` — creates ~/.config/proton-moresync/env and enables the timer.
set -u
REPO="$HOME/src/proton-moresync"
# PROTON_BACKUP_DIR can be set in ~/.config/proton-moresync/env (via make install)
# or overridden on the command line. Defaults to ~/proton-backup.
TREE="${PROTON_BACKUP_DIR:-$HOME/proton-backup}"
LOCK="$TREE/.proton-backup-sync.lock"

# Ensure the lock file can be created (tree must exist).
if [ ! -d "$TREE" ]; then
  echo "proton-backup-sync: $TREE does not exist — run one-time setup first" >&2
  exit 1
fi

exec 9>"$LOCK"
flock -n 9 || { echo "proton-backup-sync already running, skipping"; exit 0; }

# Build (incremental; Go caches, so this is fast on unchanged code) then run.
# stdin closed via </dev/null — all secrets come from keyring; no interactive prompts.
cd "$REPO" || exit 1
go build -o "$REPO/backup" ./cmd/backup || { echo "proton-backup-sync: build failed" >&2; exit 1; }
"$REPO/backup" --output-dir "$TREE" </dev/null
rc=$?

# Commit any changes in the tree.
cd "$TREE" || exit "$rc"
if [ -n "$(git status --porcelain)" ]; then
  git add -A
  git commit -q -m "proton backup $(date -Is)" || true
fi

# Push only if SSH agent has a key loaded (batch-safe, never blocks on a prompt).
#   SSH_AUTH_SOCK is inherited from the user systemd environment (set by gcr-ssh-agent at login).
#   BatchMode=yes prevents ssh itself from prompting; ConnectTimeout=10 prevents hangs.
#   Failure is silent: the commit is already local; the next run will push the backlog.
if ssh-add -l >/dev/null 2>&1; then
  GIT_SSH_COMMAND="ssh -o BatchMode=yes -o ConnectTimeout=10" \
    timeout 60 ~/src/claude-diary/git-lock-push.sh -b 'origin main' \
    || echo "proton-backup-sync: push failed or timed out — will retry next run"
fi

exit "$rc"
