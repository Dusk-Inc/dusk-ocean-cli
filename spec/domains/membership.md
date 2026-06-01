# Membership

## Description
The membership domain owns how a repository *enters, leaves, or changes place within* the workspace
registry. It covers bringing an external or on-disk repo under management, discovering the
sub-repositories an app already contains, renaming a repo, relocating a library between scopes, and
removing a repo from management.

It owns the *transitions* of the registry, not the registry itself. The shape and meaning of the
registry — kinds, code/non-code, deterministic location — belong to [[workspace]]; obtaining a repo
from its upstream (cloning) belongs to [[versioning]]; rewiring dependency edges that a rename or
move touches belongs to [[dependencies]]. Membership composes those: it decides *whether* and *where*
a repo joins or moves, then delegates the mechanics.

## Model
- **Entry transition** — a change to the set of managed repos or to a repo's identity/place:
  *adopt*, *register*, *rename*, *move*, *remove*. Each is an explicit, named act.
- **Adopt** — bring a repo that exists upstream but not yet on disk into the workspace: obtain it,
  drop a starter task configuration, and record the entry.
- **Register** — bring a repo that is *already on disk* (at an allowed location) under management:
  record the entry and drop a starter configuration, without obtaining anything.
- **Discovery** — inspecting an app's directory to find the sub-repositories (services, app
  libraries, app testing repos) it already contains, so they can be registered without naming each
  by hand.
- **Rename** — change a repo's name and propagate that change everywhere it is referenced: its
  directory, its registry entry, its cached hashes, and every dependency edge that names it.
- **Move** — relocate a *library* between scopes (app→app, app→global, global→app), propagating the
  same set of references as a rename.
- **Conflict** — a transition that would collide with an existing repo (same name/location) or that
  targets a directory whose management marker disagrees with the requested act.

## Policies

**Adopt requires the target absent; register requires it present**
- **Given** the deterministic target location for a repo
- **When** adopt runs and the location does not exist / when register runs and it does
- **Then** adopt obtains and records it, register records the already-present repo; the mismatched
  case (adopt onto an existing dir, register onto a missing dir) is refused with guidance toward the
  other command.

**A managed directory is never silently re-adopted**
- **Given** a directory that already carries a management marker
- **When** adopt or register targets it
- **Then** the act is refused with an "already registered" error rather than overwriting it.

**A transition is all-or-nothing across every reference**
- **Given** a rename or a move
- **When** it succeeds
- **Then** the directory, the registry entry, the cached hashes, and *all* dependency edges that
  named the repo are updated together; a partial update that leaves a dangling reference is not a
  valid outcome.

**Name conflicts block the transition**
- **Given** a rename or move whose destination name already belongs to another repo
- **When** the transition is attempted
- **Then** it fails before changing anything, rather than producing two repos with the same identity.

**Scope changes surface, but do not auto-resolve, broken relationships**
- **Given** a move that relocates a library other apps depended on across a scope line
- **When** the move completes
- **Then** the affected cross-scope relationships are reported as warnings; scope declarations are
  left for the human to reconcile, never silently rewritten.

**Non-code repos enter without a lifecycle**
- **Given** an infrastructure or docs repo being adopted or registered
- **When** the entry is recorded
- **Then** no starter task configuration is dropped — these repos have no build/check/install
  lifecycle to seed.

**A default identity is derived, not invented, when omitted**
- **Given** an adopt with no explicit name
- **When** the entry is created
- **Then** the name defaults to the basename of the upstream location, keeping identity tied to its
  source rather than to an arbitrary choice.

## Decisions

**Separate adopt and register around the on-disk precondition** — a short name
- **Context**: a repo may need bringing under management whether or not it is already cloned locally.
- **Decision**: split the act into *adopt* (not on disk yet → obtain it) and *register* (already on
  disk → just record it), each refusing the other's precondition.
- **Why**: the two situations have opposite expectations about an existing directory; one command
  trying to guess would either clobber local work or clone over it.
- **Rejected**: a single "ensure" command that clones-if-missing (hides whether local state was
  about to be overwritten).

**Propagate references atomically on rename/move** — a short name
- **Context**: a repo's name appears in its directory, the registry, the hash store, and other
  repos' dependency edges; a rename that updates only some leaves dangling references.
- **Decision**: treat rename and move as transitions that update every reference in lockstep and
  fail closed on conflict.
- **Why**: dangling dependency edges silently corrupt the dependency graph and the build order;
  consistency must be a precondition, not a follow-up chore.
- **Rejected**: updating the registry only and letting consumers tolerate stale names (pushes the
  inconsistency onto every reader).
