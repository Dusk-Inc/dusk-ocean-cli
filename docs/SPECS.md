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
| `WorkspaceService` | `repos/apps/<app>/services/<name>` | port, image name, dockerfile, dependencies |
| `WorkspaceLibrary` | `repos/libs/<name>` or `repos/apps/<app>/libs/<name>` | name, dependencies |
| `WorkspaceProject` | `repos/projects/<name>` | name, dependencies |
| `WorkspaceTest` | `repos/apps/<app>/testing/<name>` | name, dependencies |

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
