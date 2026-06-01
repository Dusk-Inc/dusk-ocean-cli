# Refresh

## Description
The refresh domain owns bringing the workspace — or one repo's slice of it — to a ready state in a
single pass. It obtains any managed repo that is missing on disk, then installs, builds, and checks
the code repos in dependency order, so that after a refresh everything is present and current.

It is an *orchestration* domain: it owns the bring-up policy — what is included, in what order, and
what is skipped — but delegates the actual work. Obtaining a missing repo is [[versioning]], the
order and cycle rules are [[dependencies]], the per-repo install/build/check commands are [[tasks]],
and the decision to skip unchanged work is [[caching]]. Refresh is the conductor that composes them
into one idempotent operation.

## Model
- **Refresh run** — a single bring-up pass. It has two shapes: a *whole-workspace* refresh over every
  node, and a *scoped* refresh targeting one repo and, unless suppressed, the transitive closure of
  its dependencies.
- **Inclusion set** — the repos a run acts on. A whole-workspace run includes every code node and
  clones any missing non-code repo; a scoped run includes the target's dependency closure of code
  repos only.
- **Obtain-then-prepare** — the two phases of a run: first clone whatever is missing, then install /
  build / check what is present, in dependency order.
- **Clone-only repo** — a non-code repo (infrastructure or docs): a refresh clones it if its
  directory is missing and otherwise leaves it untouched, never entering it into install/build/check.
- **Scope exclusions** — in a scoped run, the non-code repos and unrelated code repos are not
  touched, because they are not part of the target's dependency graph.
- **Hash reset** — an optional up-front clearing of the cached operation hashes to force a full
  rebuild regardless of staleness.

## Policies

**Refresh is obtain-then-prepare**
- **Given** a refresh run
- **When** it executes
- **Then** it first obtains any missing repo in its inclusion set, then installs, builds, and checks
  the code repos — never preparing a repo it has not first ensured is present.

**Work proceeds in dependency order**
- **Given** the code repos a run includes
- **When** they are installed, built, and checked
- **Then** each is processed after its dependencies, by deferring to the dependency order; a cycle is
  a hard failure for the run.

**Missing tasks and templates are skipped**
- **Given** a node with no install or build task, or a template
- **When** the run reaches it
- **Then** the absent step is skipped with a message, and templates are excluded from the run
  entirely as non-buildable.

**Non-code repos are clone-only**
- **Given** an infrastructure or docs repo whose directory is missing
- **When** a whole-workspace refresh runs
- **Then** it is cloned via the workspace recipe and then left alone; it never enters install, build,
  or check.

**A scoped run touches only the target's graph**
- **Given** a scoped refresh on one repo
- **When** it runs
- **Then** it acts on that repo and (unless dependencies are suppressed) its transitive dependency
  closure, and on nothing else — no unrelated code repos and no non-code repos.

**Refresh reconciles the hash ledger**
- **Given** a refresh run
- **When** it completes
- **Then** stale ledger entries for repos no longer in the workspace are cleaned up, and — when the
  caller requests it — the operation hashes are cleared first to force a full rebuild.

**Refresh is idempotent**
- **Given** a workspace already present and current
- **When** refresh runs again
- **Then** nothing missing is cloned and nothing unchanged is rebuilt; a no-op run is the expected
  steady state.

## Decisions

**Two shapes: whole-workspace and scoped** — a short name
- **Context**: a developer sometimes wants the entire workspace ready and sometimes only the slice
  around the one repo they are working on; doing the whole graph every time is wasteful.
- **Decision**: offer a whole-workspace refresh and a scoped refresh that walks only a target's
  dependency closure, with an option to drop even the dependencies.
- **Why**: it lets the common "get me ready to work on X" case stay cheap while still supporting a
  full bring-up, both through one idempotent operation.
- **Rejected**: only a whole-workspace refresh (forces unrelated work on every invocation).

**Non-code repos are clone-only in a refresh** — a short name
- **Context**: infrastructure and docs repos must travel with the workspace but have no build/check/
  install lifecycle.
- **Decision**: in a whole-workspace refresh, clone a missing non-code repo and then exclude it from
  every prepare step; in a scoped run, ignore non-code repos entirely.
- **Why**: it keeps these repos available locally without pretending they have a lifecycle, and keeps
  a scoped run tight to the dependency graph.
- **Rejected**: skipping non-code repos altogether (they'd never be obtained); running lifecycle
  steps on them (there is nothing to build).
