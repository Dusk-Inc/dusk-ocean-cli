# Variables

## Description
The variables domain owns how a command template becomes a concrete command: the named token
namespaces a template may reference and the rules for resolving them. Any task command — a workspace
recipe or a repo's own lifecycle task — may embed tokens, and this domain defines what each token
means, where its value comes from, and what happens when it is missing.

It owns *resolution*, not the values' homes: the workspace constants and recipes live in
[[workspace]], the per-repo task strings live in [[tasks]], and the system-supplied values are
produced by the domain that runs the task (e.g. [[scaffolding]] or the contain lifecycle in
[[tasks]]). This domain is the substitution engine they all pass their templates through.

## Model
- **Token** — a placeholder inside a command template, written with a namespace prefix and a name.
- **Namespace** — the source a token draws from. There are four, each with a distinct prefix and a
  distinct scope:
  - *environment* — values loaded once from the workspace's local environment file; the same
    regardless of which repo a task runs against.
  - *workspace variable* — user-defined constants declared once at the workspace level; global.
  - *system (ocean)* — reserved values supplied by Dusk Ocean at runtime; users cannot declare them.
  - *repo* — values drawn from a specific repo's entry; re-evaluated for each repo a task runs
    against.
- **Reserved repo names** — certain repo-namespace names (such as a repo's remote, path, and name)
  are derived automatically from the registry entry; the rest are user-supplied on the entry.
- **Reserved system values** — a fixed set produced at runtime (for example, the contain-time values
  for a service's name, port, image path, and container recipe). Users may not redefine these.
- **Resolution** — replacing every token in a template with its namespace's value to yield the final
  command. A template with no tokens resolves to itself.

## Policies

**Namespaces never fall back to one another**
- **Given** a token in some namespace
- **When** it is resolved
- **Then** its value is taken only from that namespace; a name present in another namespace is not
  substituted in its place.

**A missing value is a hard error, not an empty string**
- **Given** a token whose name is absent from its namespace at resolution time
- **When** the template is resolved
- **Then** resolution fails loudly; it never silently produces a blank, which would run a malformed
  command.

**The repo namespace is re-evaluated per target**
- **Given** one task template run against several repos
- **When** it is resolved for each
- **Then** repo-namespace tokens take that target repo's values each time, while environment,
  workspace, and system values stay constant across the run.

**System values are reserved**
- **Given** the system namespace
- **When** a user tries to declare a value in it
- **Then** that is not permitted; system values are supplied only by Dusk Ocean at runtime.

**Secrets are ordinary values, not a special concept**
- **Given** a command that needs a credential
- **When** it is written
- **Then** the credential is referenced as an environment value like any other; the domain has no
  notion of secrecy and stores nothing sensitive itself.

## Decisions

**Four prefixed namespaces with no fallback** — a short name
- **Context**: task commands need values from several sources (the shell environment, workspace
  constants, the running repo, and the tool itself), and ambiguity at the substitution site is a
  silent-error hazard.
- **Decision**: give each source a distinct prefix, resolve each token only within its namespace, and
  never fall back between them.
- **Why**: the prefix makes the source obvious at the point of use and removes any guesswork about
  which value a bare name would pick up.
- **Rejected**: a single flat variable space with precedence rules (the precedence is invisible at
  the use site and surprises authors when names collide).

**Missing keys fail loudly** — a short name
- **Context**: an unresolved token could either blank out or halt.
- **Decision**: treat any missing key as a hard error at resolution time.
- **Why**: a blank substitution produces a subtly wrong command that may do real damage; failing fast
  surfaces the misconfiguration before anything runs.
- **Rejected**: empty-string substitution (turns a config mistake into a malformed command executed
  against the repo).
