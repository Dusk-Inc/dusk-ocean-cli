# Workspace

## Description
The workspace domain owns the single authoritative record of what the monorepo contains and the
settings that apply across all of it. It knows every managed repository, what *kind* each one is,
and where each lives; it holds the workspace-global concerns that are not specific to any one repo —
the port policy, the shared variable constants, and the named task recipes.

It answers one question: *what exists, of what kind, and where.* It deliberately does **not** own
the dependency edges between repos (that is [[dependencies]]), how a repo enters or changes place in
the record (that is [[membership]]), how a repo's own lifecycle commands run (that is [[tasks]]), or
how variable tokens resolve (that is [[variables]]). Those domains read this record as their source
of truth.

## Model
- **Workspace** — the top-level container, identified by a name. It holds the registry of managed
  repositories plus the workspace-global settings (ports, variable constants, task recipes).
- **Managed repository** — an entry in the registry. Every entry has exactly one **kind**:
  *app*, *service*, *library*, *project*, *template*, *infrastructure*, or *docs*. Services and
  app-scoped libraries are nested under a parent app; the rest are top-level.
- **Code vs non-code** — a repository is one of two super-classes. *Code* repos (apps, services,
  libraries, projects, templates) carry a task configuration and participate in the build/check/
  install/contain/run lifecycle. *Non-code* repos (infrastructure, docs) carry no task configuration
  and have no lifecycle — they exist only to be obtained and otherwise left alone.
- **Library scope** — a library is either *global* (visible workspace-wide) or *app-scoped*
  (belonging to exactly one app). It is never both at once.
- **Template kind** — a template declares which kind of repo it scaffolds (a service, library,
  project, infrastructure, or docs repo). A template is a registry member but is not itself a
  buildable artifact.
- **Registration marker** — a code repo is recognized as managed by the presence of its task
  configuration at its root. The marker's presence *is* the signal of management.
- **Port policy** — the workspace owns an allowed port range and a set of reserved name→port
  assignments. Services draw their ports from this policy.
- **Variable constants and task recipes** — workspace-global named values and named command
  templates live here; they are owned by this domain but consumed by [[variables]] and [[tasks]].

## Policies

**Every managed repository has exactly one kind**
- **Given** a repository under Dusk Ocean management
- **When** it is recorded in the registry
- **Then** it is classified as exactly one kind, which fixes both its super-class (code/non-code)
  and its deterministic on-disk location.

**The registry is the single source of truth**
- **Given** a workspace
- **When** any domain needs to know whether a repo exists, its kind, or its location
- **Then** the answer comes only from the registry; nothing is managed implicitly by being present
  on disk without an entry.

**Code repos have a lifecycle; non-code repos do not**
- **Given** an infrastructure or docs repo
- **When** the workspace runs build, check, install, contain, or run
- **Then** that repo is never included — it carries no task configuration and is treated as
  clone-only.

**Templates are members but never buildable**
- **Given** a template registered in the workspace
- **When** build, check, install, run, contain, refresh, or hashing runs
- **Then** the template is excluded from all of them; it is a scaffolding source, not an artifact.

**A library is global or app-scoped, never both**
- **Given** a library entry
- **When** it is registered or moved
- **Then** it appears under the global libraries or under exactly one app, and changing that scope
  is an explicit relocation, never an implicit duplication.

**On-disk location is deterministic and not configurable**
- **Given** a repo's kind and name (and parent app where applicable)
- **When** Dusk Ocean needs its directory
- **Then** the path is derived mechanically from those facts; the layout cannot be overridden,
  because every other workflow relies on the deterministic path.

**A service's port respects the workspace port policy**
- **Given** the allowed range and the reservations
- **When** a service is assigned a port
- **Then** the port lies within the allowed range and does not collide with a reservation held by
  another name.

## Decisions

**Six categories with a code/non-code split** — a short name for the decision
- **Context**: a monorepo holds both buildable code and supporting artifacts (terraform, k8s,
  runbooks, design notes) that must be version-controlled together but have no build lifecycle.
- **Decision**: model seven registry kinds across six top-level categories, partitioned into code
  repos (with a task configuration and a lifecycle) and non-code repos (clone-only).
- **Why**: it lets one tool track everything the org owns while only ever running lifecycle
  commands against things that can actually be built, kept honest by the code/non-code line.
- **Rejected**: treating infrastructure/docs as buildable (forces empty task configs and pointless
  lifecycle passes); keeping non-code repos outside the workspace entirely (loses the single record).

**Registration marker is the presence of a task configuration** — a short name
- **Context**: Dusk Ocean must decide whether a directory it encounters is under its management.
- **Decision**: treat the presence of a repo's task configuration at the directory root as the
  marker of management; refuse to clobber a directory that already carries one.
- **Why**: a single on-disk signal makes adoption and registration idempotent and safe, with no
  separate bookkeeping to drift out of sync.
- **Rejected**: a separate lock/marker file (another artifact to maintain); inferring management
  from registry presence alone (a directory could exist without an entry and vice versa).
