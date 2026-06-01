# Caching

## Description
The caching domain owns the decision to skip work that would reproduce an unchanged result. It
computes a content hash over a repo's dependency tree, remembers the hash at which each expensive
operation last succeeded, and lets a later invocation compare the two to decide whether to run or
skip. It also owns the durable record of those hashes and keeps it honest as the workspace changes.

It owns *staleness* — the hashing and the per-operation comparison — not the operations themselves
(running build/check/contain is [[tasks]]) nor the order they run in ([[dependencies]]). It is a side
ledger that the lifecycle consults to avoid redundant work and that scripts and agents can read to
ask "what needs attention" without running anything.

## Model
- **Dependency-tree hash** — a content hash over a repo together with the transitive local
  dependencies that feed it, with declared ignore patterns excluded. Two trees with identical
  meaningful content hash the same.
- **Operation hash** — the dependency-tree hash captured at the last *successful* build, check, or
  contain for a repo. Each operation keeps its own.
- **Manifest (hash ledger)** — the durable per-repo record of the operation hashes, plus enough
  identity (kind, parent app, name) to locate the repo. It is queryable on its own.
- **Staleness** — the state where a repo's current dependency-tree hash differs from a stored
  operation hash, or that hash is missing/empty. Staleness means "the operation must run."
- **Ignore patterns** — the rules (gitignore-style) that exclude noise (build outputs, vendored
  dependencies) from the hash so only meaningful change invalidates it.

## Policies

**An unchanged hash skips the operation**
- **Given** a repo whose current dependency-tree hash equals its stored hash for an operation
- **When** that operation is requested
- **Then** the operation is skipped as already up to date.

**A changed or missing hash runs the operation**
- **Given** a current hash that differs from the stored one, or no stored hash at all
- **When** the operation is requested
- **Then** the operation runs; a missing or empty hash is always treated as stale.

**The stored hash advances only on success**
- **Given** an operation that executed
- **When** it finishes
- **Then** the new hash is recorded only if it succeeded; a failed operation leaves the stored hash
  untouched, so the next attempt still sees the work as pending.

**A repo's tree hash includes its dependencies**
- **Given** a repo that consumes local libraries
- **When** its dependency-tree hash is computed
- **Then** changes in any transitive dependency change the hash, so a consumer rebuilds when a
  library it depends on changes — not only when its own files change.

**Hashing is a pure observation**
- **Given** a request to compute or record hashes on its own
- **When** it runs
- **Then** it builds, tests, or contains nothing; it only measures and records, so the ledger can be
  refreshed or queried without side effects on artifacts.

**The ledger is pruned to the registry**
- **Given** a repo that has left the workspace
- **When** the ledger is next reconciled
- **Then** its stale hash records are removed, so the ledger never accumulates entries for repos that
  no longer exist.

**Non-buildable repos are never tracked**
- **Given** a template or a non-code repo
- **When** hashes are computed
- **Then** it is excluded from the ledger entirely — it has no buildable output whose staleness would
  mean anything.

## Decisions

**Cache key is the dependency tree, not the repo alone** — a short name
- **Context**: a repo's correctness depends on the libraries it consumes, so hashing only its own
  files would skip a rebuild after an upstream library changed.
- **Decision**: hash a repo together with its transitive local dependencies (minus ignored noise) and
  key each operation on that tree hash.
- **Why**: it makes "unchanged" mean unchanged *including inputs*, which is the only safe basis for
  skipping a build or test.
- **Rejected**: hashing the repo's own files only (misses upstream changes and skips builds that
  should run); timestamps (fragile across clones and checkouts).

**Per-operation hashes, advanced only on success** — a short name
- **Context**: build, check, and contain are independently expensive and independently stale.
- **Decision**: keep a separate stored hash per operation and update it only when that operation
  succeeds.
- **Why**: it lets each operation skip independently and guarantees a failure never marks work as
  done, so the next run retries it.
- **Rejected**: one shared hash for all operations (a check skip would wrongly imply a contain skip);
  recording the hash before the operation proves out (would mask failures).

**The ledger is a queryable side artifact** — a short name
- **Context**: scripts and agent workflows need to know which repos are stale without paying to run
  the operations.
- **Decision**: persist the operation hashes in a standalone, readable ledger that a pure hash pass
  can populate.
- **Why**: it turns "what needs attention" into a cheap read rather than an expensive dry run.
- **Rejected**: deriving staleness only as a side effect of running the lifecycle (no way to ask
  without doing).
