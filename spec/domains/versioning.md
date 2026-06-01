# Versioning

## Description
The versioning domain owns the relationship between a managed repository and its upstream version
control. Dusk Ocean is a polyrepo manager: each standalone repo may live in its own git repository,
and this domain records where that upstream is and bootstraps the version-control steps — obtaining a
repo, initializing history, provisioning a remote, and checking out a branch — through recipes the
workspace owns.

It owns *the upstream link and the VCS bootstrap recipes*, not what those recipes do internally (they
are opaque commands run by [[tasks]]), not when a missing repo is obtained during a workspace bring-up
([[refresh]]), and not the decision to adopt or register a repo ([[membership]], which calls on this
domain to clone). It is the polyrepo glue: each repo knows its own origin so the workspace need not
hand-maintain a parallel list of URLs.

## Model
- **Upstream location (remote)** — a plain string on a repo entry holding its git URL, exposed to
  recipes as a reserved token. It is the only upstream metadata tracked; no branch, auth, or other
  VCS detail rides alongside it.
- **Unknown upstream** — a sentinel value meaning the URL is not yet decided. A repo with an unknown
  upstream is still a full workspace member; it simply cannot take part in recipes that need the URL.
- **VCS recipe** — a workspace-owned command template for a version-control step: obtain (clone),
  initialize, create-remote, checkout-existing, checkout-new. The workspace owns the recipe so the
  same flow works for any host or VCS the team prefers.
- **Bootstrap** — the sequence run when a new standalone repo is created: provision its remote, then
  initialize local history, so it arrives version-controlled.
- **Shared-history repo** — an app-scoped repo (a service, app library, or app testing repo) that
  lives inside its parent app's git history and therefore is not independently bootstrapped.

## Policies

**Each standalone repo carries its own upstream**
- **Given** a service, library, project, or template
- **When** it is registered
- **Then** its upstream URL is stored on its own entry, so workspace recipes can obtain or publish it
  without a separately maintained URL list.

**An unknown upstream is a first-class state**
- **Given** a repo whose URL is not yet decided
- **When** it is recorded with the unknown-upstream sentinel
- **Then** it is a full member that simply opts out of recipes requiring the URL, rather than being
  rejected or blocked from registration.

**The workspace owns the recipe, not the host**
- **Given** a version-control step
- **When** it runs
- **Then** it runs the workspace's recipe for that step, so switching hosts or VCS is a matter of
  editing the recipe, with no host knowledge baked into the tool.

**A blank recipe is skipped, not failed**
- **Given** a VCS step whose recipe is empty
- **When** a bootstrap reaches it
- **Then** the step is skipped with a message and creation still succeeds — version-control bootstrap
  is optional.

**Credentials are supplied through the environment, never tracked**
- **Given** a recipe that needs a token to reach the upstream
- **When** it runs
- **Then** the token is referenced as an environment value inside the recipe; the domain stores no
  secret and knows nothing about how it is used.

**App-scoped repos are not independently bootstrapped**
- **Given** a service, app library, or app testing repo
- **When** it is created
- **Then** its version-control wiring is intentionally not invoked — it shares its parent app's git
  history rather than owning its own.

## Decisions

**Store only the upstream URL on the repo entry** — a short name
- **Context**: a polyrepo workspace must clone and publish many repos, but rich per-repo VCS metadata
  (branches, auth, provider quirks) is a maintenance burden and a secrets hazard.
- **Decision**: track a single plain URL per repo and nothing else; everything situational is
  expressed inside the workspace recipes and their environment values.
- **Why**: it keeps the source of truth tiny and host-agnostic while still letting recipes do
  arbitrarily host-specific things.
- **Rejected**: a structured VCS metadata block per repo (couples the tool to provider details and
  invites storing secrets next to the URL).

**Recipes are workspace-owned and host-neutral** — a short name
- **Context**: teams use different hosts and even different VCS tools; hard-coding git/GitHub would
  exclude them.
- **Decision**: express every version-control step as an editable workspace recipe with a blank
  default that is simply skipped.
- **Why**: the workspace owns its own bring-up flow and can target any host, and a team that doesn't
  want VCS bootstrap pays nothing for it.
- **Rejected**: built-in git/GitHub commands (excludes other hosts and forces a VCS choice on every
  workspace).
