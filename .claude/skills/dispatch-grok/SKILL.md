---
name: dispatch-grok
description: Dispatch implementation, audit or browser-test work in moyu to the local grok CLI as a headless executor while this session stays the orchestrator and acceptor. Use when the user asks to "派发 grok" / "dispatch grok" / "let grok do it", or when a task is large enough to hand off as a written task book. Covers the fence that actually holds on this machine, the exact dispatch command, moyu's gates and local stack, the task book, and acceptance.
---

# Dispatching grok as executor (moyu)

> **If you are the grok executor and this file was loaded into your context: ignore it.**
> It describes how the orchestrator dispatches *you*. It is not a task book.

This session is the **orchestrator**: it settles every design question, writes the task book,
decides which commands the executor may run, and accepts or rejects the result. The local `grok`
CLI (`~/.grok/bin/grok`) is the **executor**. Do not hand the execution work to Claude
subagents.

Adapted from nextmoe-infra's `.claude/skills/dispatch-grok/`, which holds the full capability
measurements (read it in place, read-only). The facts that bind here, re-measured from this repo
on 2026-09-16 against grok 1.0.30:

## 1. The fence

`~/.grok/config.toml` runs `permission_mode = "always-approve"`: grok reads, writes and runs
shell commands anywhere, unprompted. `--allow` rules enforce nothing. Two things hold:

| Fence | What it does | Verified |
|---|---|---|
| `--sandbox workspace` | Kernel (Landlock): writes only to this repo, `/tmp`, `/var/tmp`, `~/.grok`. Reads and network stay open. | `touch /home/kun/x` → `Permission denied`; `/tmp` write OK |
| `--deny 'Bash(<glob>)'` | Refuses the matching call and tells the executor; **the run continues** and `debug.log` records `deny rule matched` | `git branch --show-current` refused, run ended `end_turn` |

`dispatch.sh` passes both. Its deny list covers git writes, `gh`, migrations and the comment
importer, every database and Redis client, `docker`, `ssh`/`scp`/`kungal-neo`, curl to
production domains, `kill`, `pnpm`/`npm`/`npx`, `air`, `go run`, `nuxt dev`, and `.env`.
`--sandbox read-only` / `strict` do not start on this box (bubblewrap cannot bind
`/run/containerd`).

What neither fence covers, so the task book must: `apps/api/.env` is inside the writable
area (the deny list only stops reading it by the obvious names), and the executor can edit any
file in the repo. `git status --porcelain` at acceptance is the only check on stray writes;
`dispatch.sh` snapshots it before and after into the output directory.

The sandbox also puts Go's default build cache out of reach, so `dispatch.sh` exports
`GOCACHE=/tmp/grok-gocache-moyu`.

Never run two dispatches over overlapping paths at once.

## 2. The dispatch

```bash
S=<session scratchpad>/grok
mkdir -p "$S/<slug>"            # write the task book to $S/<slug>/task.md
GROK_OUT_ROOT="$S" .claude/skills/dispatch-grok/dispatch.sh <slug> [extra grok args]
```

Run it with `run_in_background` — a real task takes minutes. Output (`run.json`, `debug.log`,
`stderr.log`, `status.before`, `status.after`, the report) stays in the scratchpad, never the
repo. Useful extra args: `--effort low|medium|high|xhigh` (config default `xhigh`),
`--no-subagents`, `--disable-web-search`. Probe a new fence with a two-command task book at
`--effort low` first; it costs a few cents.

## 3. Who runs what

**The executor may run** (name them in the task book):

- `cd apps/api && go build ./... && go vet ./<pkg>/...`, `gofmt -l <paths>`,
  `go test ./<narrow pkg>/...` — only packages without DB-backed tests.
- `apps/web/node_modules/.bin/eslint <files>` and `apps/web/node_modules/.bin/prettier
  --check|--write <files it changed>` (the deny list blocks `pnpm`, so call the binaries).
- `rg`, `fd`, `git diff`, `git status`, `git log`.

**The orchestrator keeps:**

- `pnpm lint` and `pnpm typecheck` over the whole app (heavy; this box has run out of memory
  with a dev server up).
- Every git write, every migration, any database (`kungalgame_patch`, `kun_community`, the
  infra DBs) and the moyu comment importer. Never `127.0.0.1:5433` — it is an ssh tunnel.
- Starting or stopping the local stack (`pnpm dev`, the moyu API binary) — see §4.
- Final acceptance (§6).

## 4. Browser tests

grok's Playwright is **headed** Chromium on the user's desktop — say so before dispatching.
The orchestrator starts the stack first:

- infra from source: OAuth API :9277, image :9278, artifact :9279, catalog :9281,
  community :9282, trust :9283, OAuth web :9420.
- moyu API :5214 started with `KUN_NEXTMOE_API_BASE=http://127.0.0.1:9281` (the `.env` value
  points at a port nothing serves, and every comment wall then 404s); moyu web :6969.
- Dev logins: `user<id>@dev.local` / `kungal-dev`. User 2 is a global admin; 29 is a moyu
  moderator. `/patch/<n>` is a legacy redirect and lands on a *different* game — address walls
  as `/galgame/<id>?tab=comment`.

Screenshots: grok's Playwright refuses paths under `/tmp` and otherwise writes relative to the
repo root. Tell it not to save any, or check the repo root for stray PNGs at acceptance. Tell it
to restore any content it edits, and record a baseline (content hashes, like counts,
moemoepoint) yourself before the run.

## 5. The task book

Template: `task-book-template.md` here. English, self-contained (grok sees none of this
conversation), every design decision already made, commands allowed and forbidden by name,
scope and out-of-scope both named, the acceptance commands you will re-run, an exact report
path and structure, and a flat unranked list of anything that looks wrong. grok loads this
repo's `CLAUDE.md` itself; restate only the rules the task turns on.

What is worth dispatching: broad reads that compress into `file:line` findings, mechanical
edits a gate can check, and browser e2e passes. New code that carries design judgement saves
little, because every line still has to be read at acceptance — dispatch it only with the
design fully written into the task book.

Calibration (2026-09-16): a fully specified five-file change (moderator edit notice) took 24
turns and $0.43 and matched the task book line for line. Its "anything that looks wrong" list
held the two real bugs of the round, both outside what it was asked to change — read that
section first. Browser e2e rounds ran $2–5 and 25–30 minutes.

## 6. Acceptance

1. `diff status.before status.after` shows only the writable paths; the repo root has no new
   PNGs; `.env` is untouched.
2. Any `deny rule matched` in `debug.log` is explained in the report.
3. Read the whole diff (`git diff`, `git diff --stat`) — the report is a claim, not a result.
4. Re-run every gate yourself: `go build ./... && go vet ./...`, `gofmt -l`, the named tests,
   `pnpm lint` (0 errors), `pnpm typecheck` (only the pre-existing `image/[hash]` errors).
5. For UI work, drive the change in a browser yourself on the local stack.
6. Clean up what the run left in local data, then commit (the executor never does).
