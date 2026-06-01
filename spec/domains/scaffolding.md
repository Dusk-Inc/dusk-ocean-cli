# Scaffolding

## Description
The scaffolding domain owns the creation of new repositories from reusable sources. It stamps out a
new service, library, project, infrastructure, or docs repo from a template — copying the template's
tree with the new repo's identity substituted in — and it scaffolds an app's folder structure
directly. It also owns the interactive menu that gathers the choices a creation needs and wires the
new repo's declared dependencies in at birth.

It owns *bringing a new repo into being from a source*; once the repo exists, recording it belongs to
[[membership]], its place and kind to [[workspace]], its dependency edges to [[dependencies]], and its
git bootstrap to [[versioning]]. Scaffolding is the front of the creation flow that the others
complete.

## Model
- **Template** — a registered source repo that produces one kind of new repo (a service, library,
  project, infrastructure, or docs repo). It declares what it scaffolds and, optionally, the
  libraries the scaffolded repo should depend on.
- **Stamping** — copying a template's directory tree to the new repo's deterministic location,
  substituting placeholders (such as the new name) as it copies.
- **App scaffolding** — building an app's folder structure (its services, libraries, testing, jobs,
  docs slots) directly rather than from a template, because apps are intentionally not template-able.
- **Replacements** — the name/identity values substituted into a copied tree so the result is a
  distinct repo rather than a verbatim copy of the source.
- **Dependency propagation** — wiring the template's declared library dependencies into the freshly
  scaffolded repo at creation time, so it starts already connected.
- **Menu** — the interactive front door that elicits the kind, name, parent app (where applicable),
  template choice, and remote, then drives a creation.

## Policies

**Apps are scaffolded directly, never from a template**
- **Given** a request to create an app
- **When** scaffolding runs
- **Then** the app's structure is built directly in code; there is no app template, and asking a
  template to produce an app is refused.

**A new repo is created at its deterministic location**
- **Given** a kind and name (and parent app where applicable)
- **When** the repo is scaffolded
- **Then** it lands at the location those facts determine — the same place every other workflow will
  look for it.

**Template dependencies are propagated at creation, not inherited at runtime**
- **Given** a template that declares library dependencies
- **When** a repo is scaffolded from it
- **Then** those libraries are wired into the *new* repo as its own edges at creation time; the
  template itself is never a dependency of anything and its deps are never consulted again after
  stamping.

**A template scaffolds exactly one declared kind**
- **Given** a template
- **When** it is registered
- **Then** it declares which kind it produces, and it may only produce that kind — so a creation's
  result is predictable from the template alone.

**Creation finishes by bootstrapping the new repo**
- **Given** a freshly scaffolded standalone repo
- **When** stamping completes
- **Then** the creation flow proceeds to record it and bootstrap its version control, so a new repo
  arrives registered and git-ready rather than as a loose directory (see [[membership]],
  [[versioning]]).

## Decisions

**Templates stamp; apps are built in code** — a short name
- **Context**: most new repos are minor variations of a known shape, but an app is a composite of
  several slots (services, libs, testing, jobs, docs) rather than a single copyable tree.
- **Decision**: create services/libraries/projects/infra/docs by copying a registered template with
  replacements, and create apps by building their folder structure directly in code.
- **Why**: templating handles the common single-tree case cheaply and lets teams own their starters,
  while the app's multi-slot composite is clearer expressed as code than as a sprawling template.
- **Rejected**: an "app template" (a single tree can't capture the app's nested, multi-repo shape);
  building every repo in code (loses team-owned, customizable starters).

**Propagate template deps at scaffold time, once** — a short name
- **Context**: a scaffolded repo usually needs a known set of starter libraries, but the template is
  not itself a runtime participant.
- **Decision**: copy the template's declared deps into the new repo as its own edges at creation, and
  never treat the template as a dependency thereafter.
- **Why**: the new repo starts correctly wired without making the template a live graph node, keeping
  templates out of the build/dependency lifecycle.
- **Rejected**: leaving the new repo's deps for the developer to add by hand (error-prone, defeats
  the starter's purpose); treating the template as an ongoing dependency (drags scaffolding into the
  runtime graph).
