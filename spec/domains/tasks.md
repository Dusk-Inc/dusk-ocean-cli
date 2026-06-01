# Tasks

## Description
The tasks domain owns the execution of a repository's lifecycle commands. Dusk Ocean does not know
how any repo builds, tests, installs, containerizes, or runs — each repo declares those as plain
shell commands, and this domain resolves and runs them in the right place, in the right order, and
interprets their outcome. It also owns the workspace-level recipes (clone, init, remote creation,
checkout) as commands run against a single repo.

It owns *running a declared command and judging its result* — including turning a check's test output
into a structured result. It does not own the *order* across repos (that comes from [[dependencies]]),
*whether* a command may be skipped (that is [[caching]]), or *token substitution* inside the command
(that is [[variables]]). It is the layer that actually invokes work.

## Model
- **Lifecycle task** — a named command a code repo declares for one stage of its life: install,
  build, test (check), run, contain, add, uninstall. Any task may be left empty, meaning "this repo
  has nothing to do at this stage."
- **Workspace task** — a named command declared once at the workspace level and run against a single
  repo (for example clone, init, create-remote, checkout). The same recipe serves every repo.
- **Delegation** — a task's body is a plain shell command that hands off to whatever build system the
  repo already uses; Dusk Ocean assumes no particular tool and only needs a command it can run in the
  repo's directory.
- **Pass-through arguments** — extra arguments a caller appends to a configured command (notably for
  check), forwarded to the underlying runner after an explicit separator.
- **Outcome** — success or failure of a run. A check additionally yields a *structured test result*
  parsed from the runner's output and recorded for later inspection.
- **Containerization** — the contain task run against a staged copy of the service and its transitive
  dependencies; the staging area is always torn down afterward, whether the task succeeded or failed.
- **Pre-flight** — before a run task, the build/check/contain stages across the target's dependency
  tree are brought up to date; a failure there aborts the run.

## Policies

**An empty task is a skip, not a failure**
- **Given** a repo whose task for a stage is empty
- **When** that stage runs
- **Then** the stage is skipped with a message, and the absence is not treated as an error.

**A task runs in its own repo's directory**
- **Given** a lifecycle task
- **When** it executes
- **Then** it runs in that repo's directory, so the delegated build system resolves paths as it
  expects.

**Check builds first when needed**
- **Given** a check whose inputs have changed and whose repo also declares a build
- **When** the check runs
- **Then** the build runs first, so tests never run against stale build output.

**Check records a structured result**
- **Given** a completed check
- **When** the runner's output is captured
- **Then** it is parsed into a structured test result and recorded, so success/failure is inspectable
  beyond the raw exit status.

**Pass-through arguments require an explicit separator**
- **Given** extra arguments intended for the underlying test runner
- **When** they are supplied
- **Then** they are forwarded only after an explicit separator and appended to the configured
  command, never reinterpreted as Dusk Ocean's own flags.

**Containerization always cleans up its stage**
- **Given** a contain run that stages the service and its dependencies
- **When** the contain task completes or fails
- **Then** the staging area is removed either way; a stage is never left behind.

**Run is gated by pre-flight freshness**
- **Given** a run task for an app or service
- **When** it is invoked
- **Then** the build/check/contain stages across the dependency tree are brought current first, in
  dependency order, and any pre-flight failure aborts the run before it begins.

## Decisions

**Tasks are opaque shell commands, not tool integrations** — a short name
- **Context**: repos in the monorepo are polyglot and use many build systems (npm, make, cargo, go,
  Taskfiles); a tool that hard-codes any one of them creates lock-in.
- **Decision**: model every task as a plain shell command the repo declares and Dusk Ocean merely
  runs in the repo's directory, mixing in reserved tokens where useful.
- **Why**: it keeps Dusk Ocean build-system-agnostic — it orchestrates *when* and *in what order*
  work happens without owning *how* — so any repo can join by naming a command.
- **Rejected**: first-class integrations per build tool (heavy, brittle, and exclusionary of tools
  not yet supported).

**Run performs freshness pre-flight before executing** — a short name
- **Context**: running an app/service against stale build, test, or container output produces
  misleading results.
- **Decision**: before a run task, bring build/check/contain across the dependency tree up to date in
  dependency order and abort on any failure.
- **Why**: it guarantees a run reflects current source without the developer manually rebuilding the
  graph first.
- **Rejected**: running immediately and trusting the developer to refresh (defeats the point of a
  managed workspace and hides staleness).
