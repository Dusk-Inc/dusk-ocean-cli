# Dusk Ocean Policies
Binding technical decisions that govern how Dusk Ocean is built.

## Filesystem Injection

### Statement
All filesystem operations MUST go through an injected `afero.Fs` interface. Direct use of the `os` package for file I/O is not permitted.

### Rationale
Injecting the filesystem as a dependency allows unit tests to swap in `afero.MemMapFs` without touching disk or requiring teardown. This keeps tests fast, isolated, and side-effect-free.

### Scope
All Go source files in this project.

## Command Injection

### Statement
All external process execution MUST go through an injected `CommandRunner` interface. Direct calls to `os/exec` are not permitted in business logic.

### Rationale
Injecting `CommandRunner` allows tests to assert which commands would have been executed without running real subprocesses, and allows alternate implementations (e.g. dry-run, logged) without altering call sites.

### Scope
All Go source files in this project.

## String Literal Centralization

### Statement
Any string literal used in more than one place MUST be declared as a named constant in `src/tokens/` and referenced by name. Inline string duplication is not permitted.

### Rationale
Centralizing strings ensures that renames (file names, command names, path segments) propagate automatically. It also prevents test code from drifting out of sync with production code when shared strings change.

### Scope
All Go source files in this project.

## Dependency Scope Boundaries

### Statement
Dependency relationships between component types MUST conform to the following matrix. Relationships not listed are not permitted.

| Dependent | May Depend On |
|---|---|
| Global Library | Global Library |
| App Library | App Library (same app), Global Library, Project |
| Service | App Library (same app), Global Library, Project |
| Test | App Library (same app), Global Library, Project |
| Project | Global Library |

Self-dependency is forbidden for all types.

### Rationale
Enforcing scope boundaries at the dependency layer prevents app-scoped libraries from leaking across app boundaries and prevents circular cross-app coupling. Global libraries stay reusable precisely because their dependency surface is restricted.

## Local Runtime via Docker Compose

### Statement
Local service orchestration MUST use Docker Compose. Services MUST be defined in `docker-compose.yml` with development overrides in `docker-compose.dev.yml`. The two files MUST be merged at runtime via `docker compose -f docker-compose.yml -f docker-compose.dev.yml`.

### Rationale
Separating base configuration from dev overrides keeps production-relevant service definitions clean while allowing resource constraints and dev-specific settings to be layered on without modifying the base file.
