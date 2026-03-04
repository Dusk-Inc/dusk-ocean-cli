# Dusk Ocean CLI
Dusk Ocean is a polyglot monorepo CLI for scaffolding repositories, managing dependency wiring, and executing cached build/check workflows. Most commands are fully flag-driven and composable in scripts or agent workflows.

## Source Layout
- `src/commands`: Cobra command definitions — one file per top-level command.
- `src/functions`: All business logic, pure helpers, and domain operations.
- `src/interfaces`: Side-effect boundaries and adapters (e.g. command runners).
- `src/models`: Shared data shapes used across the codebase.
- `src/tokens`: Shared constants and enums for domain literals and kinds.

## Getting Started
Run the following from the root of your monorepo to initialize the workspace:
```bash
dusk-ocean init --name <workspace_name>
```
All CLI commands must be prefixed with `dusk-ocean`.

`init` creates:
- `ocean.workspace.json` — workspace registry (apps, libraries, projects, ports).
- `.ocean/` with `results/` and `hashes/` subfolders.
- `repos/` with `apps/` and `libs/` subdirectories.
- `repos/templates/apps/` with `services/`, `jobs/`, `libs/`, and `docs/` subfolders.
- `.gitkeep` files in newly created empty directories (except `.ocean/` folders).
- A `.gitignore` entry for `.ocean` (created or updated; no duplicates added).

## Core Concepts

### Organization
All code lives under `repos/` in one of three categories:

#### Apps (`repos/apps/<app>/`)
Full-stack or microservice-based applications. Each app contains:
- `services/` — API and backend service logic.
- `libs/` — Libraries internal to that specific application.
- `testing/` — App-scoped integration or system testing projects (polyglot).
- `jobs/` — Orchestration, deployment, and CI/CD configurations.

#### Global Libraries (`repos/libs/<name>`)
Libraries shared across the entire monorepo. Use a `-ts`, `-py`, or `-go` suffix when names collide across languages.

#### Projects (`repos/projects/<name>`)
Codebases intended for open-source distribution or external consumption. Use a language suffix when names collide.

### Workspace Configuration (`ocean.workspace.json`)
Tracks all registered apps, services, libraries, and projects along with their local dependency graphs.

```json
{
    "workspace": "workspace-name",
    "ports": {
        "allowed": { "min": 3000, "max": 3999 },
        "reserved": [
            { "name": "my-service", "port": 3001 }
        ]
    },
    "apps": [
        {
            "name": "app-a",
            "services": [
                {
                    "name": "svc-a",
                    "port": 3000,
                    "image": { "name": "app-a__svc-a", "tag": "dev" },
                    "Dockerfile": "ts.Dockerfile",
                    "deps": [
                        { "lib": "lib-a", "from": "global" }
                    ]
                }
            ],
            "libraries": [
                {
                    "name": "app-lib-a",
                    "deps": []
                }
            ],
            "testing": []
        }
    ],
    "libraries": [
        {
            "name": "lib-a",
            "deps": []
        }
    ],
    "projects": [
        {
            "name": "my-project",
            "deps": [
                { "lib": "lib-a", "from": "global" }
            ]
        }
    ]
}
```

Each `deps` entry specifies the library name (`lib`) and where it comes from (`from`). The `from` value is either `"global"`, the app name for app-scoped libs, or `"project"` for project deps.

### Repository Configuration (`ocean.config.json`)
Every service, library, and project has an `ocean.config.json` at its root. This is the language-agnostic task definition file.

```json
{
    "name": "lib-a",
    "language": "typescript",
    "type": "library",
    "tasks": {
        "build": "pnpm build",
        "test": "pnpm test",
        "install": "pnpm install",
        "add": "pnpm add --workspace {{name}}",
        "uninstall": "pnpm remove {{name}}"
    }
}
```

- `build` and `test`: invoked by `dusk-ocean build` and `dusk-ocean check`.
- `install`: invoked by `dusk-ocean install` to run the package manager (e.g. `pnpm install`).
- `add` and `uninstall`: invoked when wiring or unwiring this library as a local dependency in another repo.

### Build and Check Caching
To avoid redundant work, Dusk Ocean hashes each repository's source directory and compares it to the stored hash in `.ocean/hashes/`:
- If the hash is unchanged, the build or check step is skipped.
- If the hash has changed, the command executes and the new hash is saved.
- For `check`, if the source changed and a `build` task exists, Ocean rebuilds first, then runs the check.
- After a successful build or check, the result is recorded in `.ocean/manifest.json` (`build_run` / `check_run`).

## Commands

### `init`
Initialize a new workspace.
```bash
dusk-ocean init --name <workspace_name>
```

### `menu`
Interactive prompt interface for scaffolding and deletion. Scaffolding and repo deletion are only available through the menu — there are no flag-based equivalents.
```bash
dusk-ocean menu         # top-level command selector
dusk-ocean menu create  # scaffold a new app or library (prompts for type, name, template)
dusk-ocean menu remove  # delete a repo and clean up all references (prompts for confirmation)
```

`menu create` scaffolds:
- **App**: creates `repos/apps/<name>/` with `services/`, `jobs/`, `libs/`, and `docs/` and registers it in workspace config.
- **Library**: prompts for workspace-level (`repos/libs/<name>/`) or app-adjacent (`repos/apps/<app>/libs/<name>/`) placement, scaffolds from the selected template, and registers it in workspace config.

`menu remove` deletes the selected repo directory, runs uninstall tasks in all dependents if it is a library, and removes all references from workspace config.

Template files containing `{{placeholder}}` tokens will trigger prompts for replacement values applied to both file names and file content.

### `add` — Dependency Wiring
Wire a local dependency into another repository. This runs the library's `add` task inside the target directory and registers the relationship in workspace config.
```bash
dusk-ocean add --payload <lib-name> --target <repo-name>
```
Rules:
- A repo cannot depend on itself.
- Cross-app dependencies require both repos to share at least one scope name (see `add-scope`).

### `remove` — Dependency Unwiring
Remove a local dependency from a repository. This runs the library's `uninstall` task inside the target directory and removes the dependency entry from workspace config.
```bash
dusk-ocean remove --payload <lib-name> --target <repo-name>
```
The library must have an `uninstall` task in its `ocean.config.json` or the command will fail.

### `build`
Build a repository and its dependencies in dependency order. Skips targets whose source hash is unchanged.
```bash
dusk-ocean build service  --name <name> [--app <app>]
dusk-ocean build library  --name <name>
dusk-ocean build test     --name <name> --app <app>
dusk-ocean build project  --name <name>
```

### `check`
Run tests for a repository (builds dependencies first if needed). Writes JUnit XML results to `.ocean/results/`.
```bash
dusk-ocean check service  --name <name> [--app <app>]
dusk-ocean check library  --name <name>
dusk-ocean check test     --name <name> --app <app>
dusk-ocean check project  --name <name>
# Pass extra args to the test runner:
dusk-ocean check library --name <name> -- --watch
```
Pass-through args require `--` as a separator and are appended to the configured test command.

### `install`
Run the package manager install task for a single repository (e.g. `pnpm install`).
```bash
dusk-ocean install --library <repo-name>
```
Skips with a message if the repo has no `install` task.

### `add-scope` / `remove-scope`
Assign or remove a named scope on a repository. Scopes are the only mechanism that allows cross-app dependency relationships.
```bash
dusk-ocean add-scope    --scope-name <name> --target <repo>
dusk-ocean remove-scope --scope-name <name> --target <repo>
```
Two repos sharing at least one scope name may depend on each other regardless of app boundary or directory location. Removing a scope that active dependencies relied on prints a warning listing the affected relationships.

### `rename`
Rename a repository and propagate the change to all references (directory, workspace config, hash store, and all dependency entries).
```bash
dusk-ocean rename --repo <old-name> --new-name <new-name>
```
Fails if the new name conflicts with an existing repository or the old name is not found.

### `refresh`
Install, build, and check every node in the workspace in dependency order.
```bash
dusk-ocean refresh
dusk-ocean refresh --clear-hashes  # remove hash records first to force a full rebuild
```
Skips install or build for nodes that have no corresponding task. Fails on dependency graph cycles. Cleans up stale hash files for repos no longer in workspace config.

### `contain`
Stage a minimal Docker build context and publish a service image.
```bash
dusk-ocean contain --service <name>
dusk-ocean contain --service <name> --app <app>  # required when service name is ambiguous
```
Before running `docker build`, Ocean copies the service directory and all of its transitive local dependencies into `.ocean/stage/`, preserving their paths relative to the workspace root. The staging directory is always removed after build+push (or after a build failure).

Two workspace-root files control staging:
- `.oceanignore`: gitignore-format patterns for files to exclude (e.g. `node_modules/`). Absence is logged; no patterns are applied.
- `.oceaninclude`: one workspace-relative path per line; files are copied to the staging root (e.g. `go.work`, `pnpm-workspace.yaml`). Absence is logged; no files are copied.

### `hash`
Compute directory hashes for all registered repositories (or a single target) and write the results to `.ocean/manifest.json`. Does not build or test anything.
```bash
dusk-ocean hash                    # hash every registered repo
dusk-ocean hash --target <name>    # hash one repo, leave all other entries unchanged
```

The manifest records per-repo:

| Field | Type | Meaning |
|-------|------|---------|
| `hash` | string | SHA-256 hex digest of the source directory |
| `dirty` | bool | `true` when the hash differs from the previous record |
| `build_run` | bool | `true` when a build succeeded after the current hash was established |
| `check_run` | bool | `true` when a check succeeded after the current hash was established |
| `hashed_at` | string | RFC 3339 UTC timestamp of the last hash computation |

`build_run` and `check_run` are automatically updated by `dusk-ocean build` and `dusk-ocean check` on success. When the source hash changes, both flags are reset to `false`. This lets scripts and agent workflows query `.ocean/manifest.json` to determine which repos need attention without re-running expensive operations.

### `version`
Print the configured CLI version string.
```bash
dusk-ocean version
```

## VS Code Integration
Dusk Ocean writes JUnit XML to `.ocean/results/` after every `check` run, which VS Code test explorer extensions can consume to display results inline.

### Pytest
```json
{
    "python.testing.pytestEnabled": true,
    "python.testing.pytestPath": "dusk-ocean",
    "python.testing.pytestArgs": [
        "check", "service",
        "--name", "${workspaceFolderBasename}",
        "--internal-runner"
    ]
}
```

### Jest (via Jest extension by Orta)
```json
{
    "jest.jestCommandLine": "dusk-ocean check",
    "jest.runArgs": [
        "library",
        "--name", "${workspaceFolderBasename}",
        "--internal-runner",
        "--"
    ],
    "jest.reporters": ["default", "jest-junit"],
    "jest.junitReporterOutputDir": ".ocean/results"
}
```

### Vitest
```json
{
    "vitest.commandLine": "dusk-ocean check library --name <lib-name> --"
}
```

### Go
```json
{
    "go.alternateTools": {
        "go": "${workspaceFolder}/.ocean/bin/ocean-go-wrapper"
    }
}
```
