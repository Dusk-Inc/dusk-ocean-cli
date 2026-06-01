# Dusk Ocean Technical Spec
Dusk Ocean is a Go CLI tool for managing a polyglot monorepo. This document describes its structural components, data models, and persistence layout.

## CLI Entry Point

### Definition
Root Cobra command in `src/commands/root.go`, bootstrapped by `main.go`. Registers 11 subcommand groups and installs a `PersistentPreRunE` hook.

### Function
Entry point for all user invocations. Guards all commands except `init`, `version`, and `help` behind workspace root validation before execution.

## WorkspaceConfig

### Definition
Go struct in `src/models/workspace.go`, serialized as `ocean.workspace.json` at the workspace root.

### Function
Single source of truth for the monorepo topology. Holds the port allocation policy, and the registries for apps, global libraries, and projects.

**Key fields:**
- `Workspace` — workspace name
- `Version` — workspace version string
- `Ports` — allowed port range (`min`/`max`) and named reserved ports
- `Apps` — list of `WorkspaceApp` entries
- `Libraries` — list of global `WorkspaceLibrary` entries
- `Projects` — list of `WorkspaceProject` entries

## RepoConfig

### Definition
Go struct in `src/models/workspace.go`, serialized as `ocean.config.json` (or legacy `ocean.json`) at each repository root. Supports a flat format and a `tasks`-keyed nested format.

### Function
Per-repository metadata node. Exposes language, type classification, and the named task commands consumed by build, check, install, refresh, and contain flows.

**Key fields:**
- `Name` — repository identifier
- `Language` — e.g. `go`, `typescript`, `python`
- `Type` — one of `service`, `library`, `project`, `test`
- `Tasks` — named shell commands: `build`, `test`, `install`, `add`, `uninstall`

## Workspace Entity Models

### Definition
A family of Go structs in `src/models/workspace.go` representing each classified repository type. Embedded within `WorkspaceConfig`.

### Function
Map each component type to its directory path convention and carry the metadata fields needed by scaffolding, dependency resolution, and runtime orchestration.

| Model | Path Convention | Notable Fields |
|---|---|---|
| `WorkspaceApp` | `repos/apps/<name>` | services, app libraries, test projects |
| `WorkspaceService` | `repos/apps/<app>/services/<name>` | port, image name, dockerfile, container_file, dependencies |
| `WorkspaceLibrary` | `repos/libs/<name>` or `repos/apps/<app>/libs/<name>` | name, dependencies |
| `WorkspaceProject` | `repos/projects/<name>` | name, dependencies |
| `WorkspaceTest` | `repos/apps/<app>/testing/<name>` | name, dependencies |
| `WorkspaceTemplate` | `repos/templates/<name>` | name, kind (`service`/`library`/`project`), deps propagated at scaffold time |

## WorkspaceDep

### Definition
Go struct in `src/models/workspace.go` representing a single declared dependency relationship. Stored in each entity's `Dependencies` list inside `ocean.workspace.json`.

### Function
Links a dependent target to a named library with a source scope (`global`, `project`, or app name).

**Fields:**
- `Lib` — dependency name
- `From` — source scope

## Target

### Definition
Go struct in `src/models/workspace.go` representing a resolved workspace component at a specific filesystem path.

### Function
Identity carrier for non-interactive install and uninstall flows. Resolves the calling repository's name, type, and path from the current working directory.

## Dependency Graph

### Definition
In-memory directed acyclic graph (DAG) constructed from `ocean.workspace.json` by `BuildWorkspaceGraph()` in `src/functions/deps.go`. Nodes are typed as `service`, `app-lib`, `app-test`, `global-lib`, or `project`.

### Function
Execution order resolver for build, check, install, refresh, and library-removal flows. Sorted via Kahn's algorithm; cycle detection fails the graph before any execution begins.

## CommandRunner

### Definition
Go interface in `src/interfaces/command.go`. Production implementation: `SystemCommandRunner` wrapping `os/exec.Cmd.Run()`.

```go
type CommandRunner interface {
    Run(command *exec.Cmd) error
}
```

### Function
Adapter boundary between business logic and shell process execution. Received as a dependency by all functions that invoke external processes.

## Filesystem Interface

### Definition
`afero.Fs` from `github.com/spf13/afero`, used as the filesystem interface throughout the codebase. Production implementation: `afero.OsFs`. Test implementation: `afero.MemMapFs`.

### Function
Abstraction layer between all file I/O and the OS filesystem. Passed as a dependency into scaffolding, config reads/writes, hash I/O, and result writes.

## Hash Store

### Definition
Directory tree at `.ocean/hashes/` in the workspace root. One plain-text file per target per operation type.

```
.ocean/hashes/
├── build/<kind>/<name>
└── check/<kind>/<name>
```

### Function
Persistence layer for SHA256 source directory hashes. Read before and written after each build or check execution to support change detection.

## Test Results Store

### Definition
Directory tree at `.ocean/results/` mirroring the `repos/` path structure. One JUnit XML file per test target.

```
.ocean/results/repos/apps/<app>/services/<svc>/results.xml
.ocean/results/repos/apps/<app>/libs/<lib>/results.xml
.ocean/results/repos/apps/<app>/testing/<test>/results.xml
.ocean/results/repos/libs/<lib>/results.xml
.ocean/results/repos/projects/<project>/results.xml
```

### Function
Persistence layer for test execution output. Written after each `check` run; consumed by VS Code Test Explorer via JUnit XML auto-discovery.

## Docker Compose Files

### Definition
Up to three YAML files per app directory, managed by Dusk Ocean and parsed via `gopkg.in/yaml.v3`: `docker-compose.yml`, `docker-compose.dev.yml`, and optionally `docker-compose.hashi.yml`.

### Function
Runtime topology declaration for each app. Base file carries service image references and port mappings; dev file carries resource limits and reservations.

## Scaffold Templates

### Definition
Template directories under `repos/templates/` at the workspace root. Files may contain `{{placeholder}}` tokens in names and contents.

### Function
Source material for `add` commands. Copied recursively to scaffold destinations with token substitution applied before copy.

## Tokens

### Definition
String constant packages in `src/tokens/`. Three files: `commands.go`, `files.go`, `paths.go`.

### Function
Centralized string literal store for command names, file names, and path segments shared across the codebase.

## VariableContext

### Definition
Go struct in `src/functions/variables.go` carrying four `map[string]string` fields — `Env`, `Var`, `Ocean`, `Repo` — one per substitution namespace.

### Function
Aggregates the values consulted by `Substitute`. Each map is keyed by the bare variable name (no namespace prefix) and is populated by callers before substitution runs.

## Substitute

### Definition
Top-level function in `src/functions/variables.go`. Uses a regex-driven `ReplaceAllStringFunc` against `{{ns:name}}` tokens.

### Function
Replaces every recognized token in a template string with the value from the matching namespace in `VariableContext`. Strict: an unknown namespace or a missing key in a known namespace surfaces a hard error rather than producing a silent empty string. Used by the workspace task executor; the contain command keeps its own literal `ReplaceAll` path to preserve lenient tolerance of unknown tokens in contain tasks.

## LoadEnvFile

### Definition
Function in `src/functions/variables.go`. Reads `.env` from the workspace root via the supplied `afero.Fs`.

### Function
Parses simple `KEY=VALUE` pairs (with `#` comments and blank lines tolerated) and returns them as a map. A missing file is logged via the writer argument and yields an empty map without raising an error, matching the project's "no silent defaults" guidance.

## BuildRepoVariables

### Definition
Function in `src/functions/variables.go` that walks the workspace config to find a repo entry by kind and name.

### Function
Returns the full `{{repo:*}}` map for that entry: reserved fields auto-derived from the workspace model (name, kind, path, scopes, remote, plus per-kind extras like port and image fields for services), with user-declared `variables` merged on top. A user variable that collides with a reserved field name is rejected with a hard error.

## RepoKind helpers

### Definition
`ResolveRepoPath` and `ValidateRepoKindFlags` in `src/functions/repo_kind.go`, plus the `RepoKindProject`, `RepoKindLibrary`, `RepoKindApp`, and `RepoKindService` constants in `src/tokens/workspace.go`.

### Function
Centralize the deterministic on-disk layout for adopted/registered repos and the flag-validation rules: `service` requires `--app`, `library` accepts `--app` optionally (presence flips global → app-scoped), `project`, `app`, and `template` reject `--app`. `template` additionally requires `--template-kind` (one of `service`, `library`, `project` — `app` is rejected because apps are not template-able). Used by both `AdoptRepo` and `RegisterRepo` to keep their behaviors consistent.

## WriteStarterRepoConfig

### Definition
Function in `src/functions/starter_config.go` that writes a minimal `ocean.config.json` at a given repo path.

### Function
Drops the empty-task-strings shape used by adopt/register. Refuses to overwrite an existing `ocean.config.json` so a developer's existing config is never silently clobbered.

## AdoptRepo

### Definition
Function in `src/functions/adopt.go` invoked by `dusk-ocean adopt` and the menu adopt flow.

### Function
Validates flags, checks the deterministic target path, clones the supplied remote URL via the package-level `gitClone` injection point (stubbable in tests), writes a starter `ocean.config.json`, and registers the new entry in `ocean.workspace.json` with the remote URL populated. Handles the three precondition states (target absent → proceed; target exists without config → suggest register; target exists with config → already-registered error).

## RegisterRepo

### Definition
Function in `src/functions/register.go` invoked by `dusk-ocean register` and the menu register flow.

### Function
Mirror of `AdoptRepo` minus the clone step. Operates on a directory the developer has already placed at the deterministic workspace path. Writes a starter `ocean.config.json` and adds a workspace entry with `remote` set from `--remote` or the literal string `"None"` when `--remote` is omitted.

## RunWorkspaceTask

### Definition
Function in `src/functions/workspace_tasks.go`. Public entry point delegates to `RunWorkspaceTaskAt`, which is the test-friendly core that takes the workspace root explicitly.

### Function
Resolves the named workspace task from `config.Tasks`, infers the target repo's kind via `ResolveRepoKindByName`, builds the full `VariableContext` (env from `LoadEnvFile`, var from `LoadWorkspaceVariables`, repo from `BuildRepoVariables`, ocean intentionally empty), substitutes the template via `Substitute`, and executes the resulting command via the package-level `runShell` injection point (stubbable in tests). Single-repo execution only; iteration is intentionally deferred.

## WorkspaceTemplate registry

### Definition
Helpers in `src/functions/templates.go` (`AddTemplateToWorkspace`, `RemoveTemplateFromWorkspace`, `FindTemplateIndex`, `FindTemplatesByKind`, `FindTemplatesByKinds`, `ValidateTemplateDepsForTarget`, `PropagateTemplateDeps`) backed by `WorkspaceConfig.Templates` and the `WorkspaceTemplate` model.

### Function
Manages the workspace registry of scaffold templates. Each `WorkspaceTemplate` carries a `kind` (`service`, `library`, or `project`) and a `deps` list. Apps are intentionally not template-able — `ValidateTemplateKind`, `FindTemplatesByKind`, and `FindTemplatesByKinds` all reject the `app` kind. Templates are excluded from `BuildWorkspaceGraph`/`CollectWorkspaceNodes`, so they never participate in build/check/refresh/contain/hash flows. `ListTemplatesByType` accepts one or more kinds, consults `Templates` first via `FindTemplatesByKinds`, and falls back to a filesystem walk under `repos/templates/` for unregistered template directories so the dev loop of "drop a folder, scaffold from it" still works. Service and library scaffolds invoke it with their own kind only; project scaffolds invoke it with `service`, `library`, and `project` so a project may seed from any non-app template kind (REQ 19.6.1).

## PropagateTemplateDeps

### Definition
Function in `src/functions/templates.go`, called from each `add*Cmd` flow in `src/commands/add.go` after the freshly-scaffolded repo is registered in workspace config.

### Function
Walks the template's registered `deps` list and wires each entry into the new repo by delegating to `WireLocalDependency`, so the same scope/cycle/flow checks and `add` task execution apply as if the user had run `dusk-ocean add` by hand. The companion `ValidateTemplateDepsForTarget` runs **before** any files are copied — using the same `resolveDependency` + `validateInstallFlow` rules against a synthetic Target — so an illegal propagation aborts cleanly with no half-scaffolded state on disk.
