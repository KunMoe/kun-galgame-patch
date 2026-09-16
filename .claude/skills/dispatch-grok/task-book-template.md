# Task: <one line, imperative, what gets built>

You are the **executor**. The orchestrator wrote this task book; it is self-contained.
You cannot see the orchestrator's conversation. Everything you need is below.

## Context

Repository: `/home/kun/Desktop/code/website/kun-galgame-patch` (moyu; your working directory).
Branch: `<branch>` at `<short sha>`. `apps/api` is Go (Fiber v3 + GORM), `apps/web` is Nuxt 4.

<Where the relevant code lives — exact paths. What it does today. Why it is changing.
Every prior decision this task depends on, stated inline. If the reader would have to ask
"why this way and not the obvious way", answer it here.>

## Your environment

- You run under a kernel sandbox: you may write inside this repository, `/tmp` and `~/.grok`,
  and nowhere else. A write outside those fails with `Permission denied` — that is the sandbox,
  not a bug, and it is not to be worked around.
- Some commands are refused by policy (git writes, databases, package managers, services,
  production hosts). A refusal is final: do not retry it another way; note it in section 4.
- **Do not read, print or edit `apps/api/.env` or any credential file.**
- You may run **only** these commands:
  - `<exact list — e.g. cd apps/api && go build ./... ; go vet ./internal/<pkg>/... ;
    gofmt -l <paths> ; go test ./internal/<pkg>/... ;
    apps/web/node_modules/.bin/eslint <files> ; apps/web/node_modules/.bin/prettier --write <files you changed> ;
    rg ; fd ; git diff ; git status>`
  - Run them. Code that does not compile is not finished work. The orchestrator re-runs every
    one of them, plus `pnpm lint` and `pnpm typecheck`, at acceptance.
- The repository `CLAUDE.md` is already in your context. Its iron rules and its comment policy
  bind you.
- <Delete if not applicable:> Do not use web search, the browser tools, or any MCP tool.

## Scope

1. <numbered, concrete, each independently checkable>
2. …

## Out of scope

- <what a helpful executor would otherwise wander into>
- Renaming, reformatting, or refactoring anything not named in Scope.
- Migrations, docs under `docs/{oauth,image_service,artifact}/` (infra mirrors), KunUI sources.

## Precedent to follow

<file:line of existing code that already does this correctly. Say what to copy — the shape,
the error handling, the naming, the UI wording — and what not to.>

## Acceptance criteria

Run the ones your command list covers before you write the report:

- `<exact command>` → `<expected output>`
- Test `<TestName>` in `<file>` must pass.
- `git status --porcelain` shows changes **only** under the writable paths below.

## Report

Write your report to this exact absolute path:

    <GROK_OUT_ROOT>/<slug>/report.md

Structure:

```
# Report: <task>

## 1. What I changed
(file:line per change, one line each, what and why)

## 2. Anything that looks wrong — in scope or not
(every one, at the same weight, with file:line and the quoted line. Report it; do not fix it.)

## 3. Mechanics I chose
(any decision the task book left to the code — what you picked and the precedent you followed)

## 4. Deviations from the task book, and refused commands
(if none, write "None.")

## 5. Gates I ran, and what I could not verify
(each command: exact command and result. Then what you could not settle, and what the
 orchestrator should check.)
```

Your final stdout message: one short paragraph, the report path plus a one-line status.

## Discipline

- Writable paths — **exactly** these, nothing else:
  - `<path 1>`
  - `<path 2>`
  - the report path above
- Forbidden: any git command that writes; any database, migration or importer; starting or
  stopping any service; `pnpm` / `npm` / `npx`; any shell command outside the list above;
  editing outside the writable paths; saving screenshots into the repository.
- **Report, don't work around.** If something is missing, contradictory or blocked, stop and
  write it in section 4 or 5.
- **Do not rank, score or filter findings.** Report every one flat, at equal weight.
- <Delete unless the task is a search, audit or census:> **Include a positive control** — what
  you checked that came back clean, with counts, so a zero can be believed.
