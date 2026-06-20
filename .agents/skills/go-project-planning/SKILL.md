---
name: go-project-planning
description: Use this skill at the START of a new Go project or before implementing a major new feature in an existing Go codebase, BEFORE any code is scaffolded. Drives a structured planning pass — requirements clarification, package/module architecture, interface-first design, data modeling, dependency selection, concurrency strategy, risk identification, and test strategy — and produces a short design doc. Not for code style, linting, performance tuning, or implementation guidance (see golang-naming, golang-performance, go-testing skills for those) — this skill owns the phase BEFORE code is written, not the code itself. Trigger phrases: "let's plan this Go project", "help me design this service", "before we start coding", "what should the architecture look like", "new Go app/service/CLI/library".
---

# Go Project Planning

## Purpose

Force a planning pass before code exists. Go rewards up-front decisions on package
boundaries, interfaces, and concurrency model — these are expensive to retrofit later.
This skill is a procedure, not a style guide: it tells the agent what to ask, what to
draft, and what order to do it in. Output is a short design doc, not code.

Do NOT start writing implementation code, `go.mod`, or directory scaffolding until
Phase 5 is explicitly confirmed by the user.

## When NOT to use this

- Mid-implementation bug fixes, refactors of existing logic, style/lint questions →
  not this skill.
- Trivial scripts / one-off tools with no real architecture decisions → skip to coding,
  this skill adds overhead with no payoff.

## Procedure

Work through phases in order. Each phase ends with a short written artifact appended
to a running `DESIGN.md`. Do not skip phases, but phases can be brief (a few bullets)
for small projects — match the depth to the project's actual complexity.

### Phase 1 — Scope and non-goals

Ask (don't assume):
- What does this do, in one paragraph?
- Who/what calls it (human CLI user, HTTP clients, another service, a cron job)?
- Explicit non-goals — what is this deliberately NOT solving right now?
- Is this a library, CLI, long-running service, or batch job? (This decision drives
  almost everything else in Go: a library should minimize dependencies and avoid
  global state; a service needs graceful shutdown and observability from day one.)

Output: 5-10 line "Scope" section.

### Phase 2 — Architecture sketch

Decide before writing code:
- Module layout: justify use of `cmd/`, `internal/`, `pkg/` — don't apply them by
  habit. `internal/` for anything not meant to be imported externally; `pkg/` only
  if external consumers genuinely need importable packages; small projects often
  need neither.
- Package boundaries: propose packages by responsibility, not by layer (avoid
  `models/`, `utils/`, `helpers/` dumping grounds). Each package should have a clear
  one-sentence reason to exist.
- Dependency direction: draw (in text/ASCII) which packages import which. Flag any
  cycle risk now — Go will refuse to compile cycles, so this must be resolved on
  paper before coding.
- Where do interfaces live? Go convention: define interfaces at the **consumer**,
  not the implementer, unless it's a small, genuinely shared abstraction. State
  this explicitly per major interface.

Output: package list with one-line responsibility each, plus an import-direction
sketch.

### Phase 3 — Concurrency model (skip if purely sequential)

If the project has any concurrent work (servers, fan-out calls, pipelines):
- Pick a model explicitly: worker pool, pipeline with channels, errgroup,
  one-goroutine-per-request, or none. Don't leave this implicit.
- Identify shared mutable state up front and how it's protected (mutex, channel
  ownership, atomic, or — preferably — avoided by design).
- Decide cancellation/timeout propagation strategy (`context.Context` usage) now;
  retrofitting context plumbing later touches every signature.

Output: 3-5 sentences naming the concurrency model and shared-state strategy.

### Phase 4 — Data & interface design

- Draft core struct definitions (fields + types only, no methods yet).
- Draft the key interface signatures for the system's main abstractions. Ask: "if I
  were calling this from a test, what would I want this interface to look like?"
- If there's persistence, sketch how structs map to storage (table shape, JSON
  contract, or serialization format) before writing any DB code.
- List 2-3 alternative designs for the trickiest struct/interface and state the
  tradeoff reasoning for the chosen one, in 1-2 sentences each — don't just present
  one option as obvious.

Output: draft structs/interfaces (as Go-like pseudocode is fine), plus tradeoff notes.

### Phase 5 — Dependencies, risks, and test strategy

- Dependency choices: for each non-trivial third-party dependency (router, ORM,
  config lib, etc.), name 2 alternatives considered and why the choice was made.
  Default bias: prefer stdlib unless a clear concrete need justifies a dependency.
- Risk list: enumerate the 2-5 riskiest or least-understood parts of the system —
  these should be prototyped/spiked first, not left for last.
- Test strategy: table-driven test conventions, what needs unit vs integration
  coverage, what needs fakes/mocks (and which interfaces from Phase 4 exist
  specifically to enable that).

Output: dependency table, risk list, test strategy paragraph.

## Final step

Compile Phases 1-5 into a single `DESIGN.md` and present it to the user for
confirmation before any scaffolding or implementation begins. Explicitly ask:
"Does this match what you want, or should we revisit any phase?"

Only after explicit confirmation should the agent proceed to scaffolding
(`go mod init`, directory creation, initial files).
