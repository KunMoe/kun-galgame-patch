#!/usr/bin/env bash
# Dispatch one grok executor run against this repo. See SKILL.md in this directory.
#
#   GROK_OUT_ROOT=<session scratchpad>/grok dispatch.sh <slug> [extra grok args...]
#
# Expects <GROK_OUT_ROOT>/<slug>/task.md. Writes run.json, stderr.log, debug.log and
# the before/after `git status --porcelain` snapshots beside it.
#
# Knobs: GROK_SANDBOX_PROFILE (default workspace), GROK_MAX_TURNS (default 200),
# GROK_TIMEOUT seconds (default 7200).
#
# Run this in the background: a real dispatch takes minutes.
set -euo pipefail

slug="${1:?usage: dispatch.sh <slug> [extra grok args...]}"
shift

root="${GROK_OUT_ROOT:?set GROK_OUT_ROOT to a directory under the session scratchpad}"
out="$root/$slug"
[ -f "$out/task.md" ] || { echo "dispatch: missing $out/task.md" >&2; exit 2; }

repo="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
cd "$repo"

# The workspace sandbox lets grok write only the repo, /tmp, /var/tmp and ~/.grok,
# and Go's default build cache (~/.cache/go-build) is outside all four.
export GOCACHE="${GROK_GOCACHE:-/tmp/grok-gocache-moyu}"

# Always-approve config: the sandbox fences writes, these rules fence shared state.
# A matching call is refused and the executor is told why; the run goes on (grok
# 1.0.30), so a refusal only shows in debug.log and is printed below. Tokens that
# never appear in a search get a leading `*`; common words match only as the
# command, or every `rg` for them would be refused too.
deny=(
  --deny 'Bash(*git commit*)' --deny 'Bash(*git push*)' --deny 'Bash(*git checkout*)'
  --deny 'Bash(*git switch*)' --deny 'Bash(*git reset*)' --deny 'Bash(*git restore*)'
  --deny 'Bash(*git stash*)' --deny 'Bash(*git rebase*)' --deny 'Bash(*git merge*)'
  --deny 'Bash(*git branch*)' --deny 'Bash(*git worktree*)' --deny 'Bash(*git add*)'
  --deny 'Bash(*git clean*)' --deny 'Bash(*git tag*)' --deny 'Bash(gh *)'
  --deny 'Bash(*cmd/migrate*)' --deny 'Bash(*import-moyu-comments*)'
  --deny 'Bash(*psql*)' --deny 'Bash(*pg_dump*)' --deny 'Bash(*pg_restore*)'
  --deny 'Bash(*redis-cli*)' --deny 'Bash(*5433*)' --deny 'Bash(*kungal-neo*)'
  --deny 'Bash(docker*)' --deny 'Bash(ssh *)' --deny 'Bash(scp *)'
  --deny 'Bash(*curl*moyu.moe*)' --deny 'Bash(*curl*kungal.com*)' --deny 'Bash(*curl*nextmoe.com*)'
  --deny 'Bash(kill *)' --deny 'Bash(*pkill *)' --deny 'Bash(*killall *)' --deny 'Bash(*fuser *)'
  --deny 'Bash(*pnpm *)' --deny 'Bash(npm *)' --deny 'Bash(*npx *)'
  --deny 'Bash(air*)' --deny 'Bash(*go run *)' --deny 'Bash(*nuxt dev*)' --deny 'Bash(*nuxi dev*)'
  --deny 'Bash(*/.env)' --deny 'Bash(*/.env *)' --deny 'Bash(* .env)' --deny 'Bash(* .env *)'
  --deny 'Read(**/.env)'
)

git status --porcelain >"$out/status.before"

timeout "${GROK_TIMEOUT:-7200}" grok --prompt-file "$out/task.md" \
  --sandbox "${GROK_SANDBOX_PROFILE:-workspace}" \
  "${deny[@]}" \
  --allow "Write($out/**)" \
  --allow "Read($out/**)" \
  --output-format json \
  --max-turns "${GROK_MAX_TURNS:-200}" \
  --debug-file "$out/debug.log" \
  "$@" \
  >"$out/run.json" 2>"$out/stderr.log" || true

git status --porcelain >"$out/status.after"

if [ ! -s "$out/run.json" ]; then
  echo "dispatch: grok produced no JSON; see $out/stderr.log" >&2
  tail -c 2000 "$out/stderr.log" >&2 || true
  exit 1
fi

stop=$(jq -r '.stopReason // "?"' "$out/run.json")
turns=$(jq -r '.num_turns // "?"' "$out/run.json")
cost=$(jq -r '.total_cost_usd // "?"' "$out/run.json")
printf 'stopReason=%s turns=%s cost_usd=%s out=%s\n' "$stop" "$turns" "$cost" "$out"
echo '--- git status changes (before -> after) ---'
diff "$out/status.before" "$out/status.after" || true
if rg -q 'deny rule matched' "$out/debug.log" 2>/dev/null; then
  echo 'dispatch: a --deny rule refused at least one call; the report must say what it tried' >&2
fi

if [ "$stop" != "end_turn" ]; then
  if [ "$turns" -ge "${GROK_MAX_TURNS:-200}" ] 2>/dev/null; then
    echo 'dispatch: turn budget exhausted; raise GROK_MAX_TURNS or split the task' >&2
  else
    echo "dispatch: run ended as '$stop'; inspect $out/debug.log" >&2
  fi
  exit 1
fi
