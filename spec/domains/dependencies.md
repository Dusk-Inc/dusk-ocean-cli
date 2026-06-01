# Dependencies

## Description
The dependencies domain owns the local dependency graph among code repositories — which repo
consumes which library, how those edges are constrained, and the order work must happen in as a
result. It covers wiring and unwiring an edge, the scope rules that permit cross-app edges, deriving
a dependency-ordered traversal, and rejecting cycles.

It owns the *edges and their ordering*, not the *nodes* (the repos themselves are owned by
[[workspace]] and [[membership]]) and not the *execution* that walks the order (that is [[tasks]] and
[[refresh]], which ask this domain for the order and then run something at each node). The graph it
maintains is the spine the build/check/install/contain lifecycle is sequenced along.

## Model
- **Node** — a code repo as a participant in the graph: a service, an app library, a global library,
  a project, or an app testing repo. Non-code repos are not nodes; templates are not nodes.
- **Edge (dependency)** — a directed "consumes" relationship from a repo to a library. Each edge
  names the library and its *origin*: a global library, a specific app's library, or a project's
  library. The origin disambiguates which library is meant when names repeat across scopes.
- **Dependency order** — a linearization of a node and its transitive dependencies such that every
  dependency comes before the repo that consumes it. It is the order build/check/install run in.
- **Cycle** — a path of edges that returns to its start. A cycle makes dependency order undefined and
  is therefore forbidden.
- **Scope** — a named tag on a repo. Two repos may form a cross-app edge only if they share at least
  one scope name; scopes are the sole mechanism that crosses the app boundary.
- **Wiring / unwiring** — adding or removing an edge. Adding runs the library's own "add" step inside
  the consumer and records the edge; removing runs the library's "uninstall" step and drops the edge.

## Policies

**No repo depends on itself**
- **Given** a wiring request whose payload and target are the same repo
- **When** the edge would be added
- **Then** it is refused — a self-edge is meaningless and would manufacture a trivial cycle.

**The graph stays acyclic**
- **Given** the set of dependency edges
- **When** a dependency order is derived (e.g. before build/check/install or a refresh)
- **Then** any cycle is detected and reported as a hard failure rather than producing an arbitrary
  order.

**Only libraries are dependable**
- **Given** a wiring request
- **When** the payload is a project or a template
- **Then** it is refused: projects cannot be consumed by apps/services/libraries, and templates can
  never be a dependency (their declared deps are propagated only at scaffold time, by [[scaffolding]]).

**Cross-app edges require a shared scope**
- **Given** two repos in different apps
- **When** an edge is requested between them
- **Then** it is permitted only if they share at least one scope name; otherwise it is refused.

**Removing a scope warns about what it breaks**
- **Given** a scope removal that some active edge relied on to be legal
- **When** the scope is removed
- **Then** the affected relationships are listed as a warning; the edges are not silently deleted and
  the scope is not silently retained.

**Work is sequenced in dependency order**
- **Given** a target node and its transitive dependencies
- **When** build, check, or install runs for that target
- **Then** each dependency is processed before the repo that consumes it, and an already-processed
  node is not processed twice within one run.

**Unwiring requires an uninstall step**
- **Given** a repo whose library has no "uninstall" step defined
- **When** an edge into it is removed
- **Then** the removal fails rather than leaving the consumer's package state inconsistent with the
  dropped edge.

## Decisions

**Edges carry an explicit origin** — a short name
- **Context**: library names repeat across scopes (a global `lib-a` and an app-scoped `lib-a` can
  coexist), so a bare name does not identify a dependency.
- **Decision**: every edge records the library name *and* where it comes from (global / a named app
  / a project), so the edge is unambiguous.
- **Why**: it lets the graph resolve the right library deterministically and keeps the build order
  correct when names collide.
- **Rejected**: globally-unique library names (forces awkward renaming and leaks scope into identity).

**Scopes are the only cross-app channel** — a short name
- **Context**: app boundaries exist to keep apps independent, but some libraries legitimately need to
  be shared across apps.
- **Decision**: forbid cross-app edges by default and permit them only between repos that share an
  explicit, named scope.
- **Why**: it keeps the default safe (apps stay decoupled) while making every intentional crossing
  visible and auditable as a shared scope.
- **Rejected**: allowing any cross-app edge freely (erodes app isolation); a global allow-list
  (less local, harder to reason about per-relationship).

**Projects and templates are not dependable** — a short name
- **Context**: projects are self-contained leaf tools and templates are scaffolding sources, not
  reusable runtime code.
- **Decision**: only libraries may be the payload of a dependency edge; projects and templates are
  rejected as dependencies.
- **Why**: it preserves the meaning of each kind — projects consume but are not consumed, templates
  only seed deps at creation time — and keeps the graph to genuinely shared code.
- **Rejected**: letting projects be depended on (blurs the leaf-tool role and invites cycles).
