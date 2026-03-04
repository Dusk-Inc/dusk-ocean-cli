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
Standalone repositories for self-contained tools, CLIs, research repos, or other non-library, non-application work. Projects participate in the standard build/check/install workflow and may depend on global libraries, but cannot be used as dependencies by other repositories. Use a language suffix when names collide.

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
                    "container_file": "ts.Dockerfile",
                    "image_path": "registry.example.com/app-a/svc-a",
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

Each `deps` entry specifies the library name (`lib`) and where it comes from (`from`). The `from` value is either `"global"` for workspace-level libraries or the app name for app-scoped libraries. Projects may depend on global libraries but cannot themselves be used as dependencies.

### Repository Configuration (`ocean.config.json`)
Every app, service, library, and project has an `ocean.config.json` at its root. This is the language-agnostic task definition file.

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
        "uninstall": "pnpm remove {{name}}",
        "contain": "",
        "run": ""
    }
}
```

- `build` and `test`: invoked by `dusk-ocean build` and `dusk-ocean check`.
- `install`: invoked by `dusk-ocean install` to run the package manager (e.g. `pnpm install`).
- `add` and `uninstall`: invoked when wiring or unwiring this library as a local dependency in another repo.
- `contain`: invoked by `dusk-ocean contain` to build and publish a service container image.
- `run`: invoked by `dusk-ocean run` to execute the app or service after pre-flight checks.

### Build, Check, and Contain Caching
To avoid redundant work, Dusk Ocean computes a dependency-tree hash for each repository and compares it to the stored per-operation hash in `.ocean/manifest.json`:
- If the dependency-tree hash matches the stored `build_hash`, `check_hash`, or `contain_hash`, the corresponding operation is skipped.
- If the hash differs (or is missing), the operation executes and the new hash is saved on success.
- For `check`, if the hash changed and a `build` task exists, Ocean rebuilds first, then runs the check.
- The `contain` operation uses the same hash-based skip logic applied to the service's dependency tree.

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
- Projects cannot be used as dependencies by apps, services, or libraries.
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
Stage a minimal build context and execute the service's `contain` task to build and publish a container image.
```bash
dusk-ocean contain --service <name>
dusk-ocean contain --service <name> --app <app>  # required when service name is ambiguous
```
Rather than enforcing a specific containerization tool, Dusk Ocean executes the service's `contain` task (defined in `ocean.config.json`) after staging the build context. Before executing, Dusk Ocean substitutes reserved placeholders (`{{ocean:service_name}}`, `{{ocean:port}}`, `{{ocean:image_path}}`, `{{ocean:container_file}}`) in the task command with runtime values. Ocean copies the service directory and all of its transitive local dependencies into `.ocean/stage/`, preserving their paths relative to the workspace root. The staging directory is always removed after the contain task completes or fails.

Each service may declare `container_file` (path to the container build recipe) and `image_path` (full registry path for the built image) in workspace config. A bare filename for `container_file` resolves to `repos/containers/<name>`; a value with path separators is treated as workspace-root-relative.

Dusk Ocean computes a dependency-tree hash before contain; if the hash matches the stored `contain_hash` in the manifest, the build is skipped.

Two workspace-root files control staging:
- `.oceanignore`: gitignore-format patterns for files to exclude (e.g. `node_modules/`). Absence is logged; no patterns are applied.
- `.oceaninclude`: one workspace-relative path per line; files are copied to the staging root (e.g. `go.work`, `pnpm-workspace.yaml`). Absence is logged; no files are copied.

### `move`
Relocate a library repository from one location to another within the workspace. Updates the physical directory, workspace config, hash store paths, and all dependency references.
```bash
dusk-ocean move --library <name> --from-app <app> --to-app <app>    # move between apps
dusk-ocean move --library <name> --from-app <app> --to-global       # move app lib to global
dusk-ocean move --library <name> --from-global --to-app <app>       # move global lib to app
```
Fails if the destination name conflicts with an existing repository. When a move creates scope violations (e.g. moving a global library into an app scope when other apps depended on it), Dusk Ocean prints a warning listing the affected relationships. Scope declarations are not altered automatically.

### `run`
Execute a user-defined `run` task for an app or service. Before running, Dusk Ocean performs hash-based pre-flight checks for build, check, and contain across all repos in the target's dependency tree. Stale tasks are executed in dependency order (build → check → contain) before the run task begins. If any pre-flight task fails, the run is aborted.
```bash
dusk-ocean run app     --name <app-name>
dusk-ocean run service --name <service-name> [--app <app>]
```
For an app, pre-flight checks are performed for each service in the app. If no `run` task is defined, the command skips with a message.

### `hash`
Compute directory hashes for all registered repositories (or a single target) and write the results to `.ocean/manifest.json`. Does not build or test anything.
```bash
dusk-ocean hash                    # hash every registered repo
dusk-ocean hash --target <name>    # hash one repo, leave all other entries unchanged
```

The manifest records per-repo:

| Field | Type | Meaning |
|-------|------|---------|
| `kind` | string | Repository type (e.g. `"service"`, `"library"`, `"project"`) |
| `app` | string | Parent app name (empty for global libraries and projects) |
| `name` | string | Repository name |
| `build_hash` | string | Dependency-tree hash at last successful build |
| `check_hash` | string | Dependency-tree hash at last successful check |
| `contain_hash` | string | Dependency-tree hash at last successful contain |

`build_hash`, `check_hash`, and `contain_hash` are set by `dusk-ocean build`, `dusk-ocean check`, and `dusk-ocean contain` respectively on success. Staleness is determined by comparing the current dependency-tree hash against the stored operation hash; a missing or empty hash is treated as stale. This lets scripts and agent workflows query `.ocean/manifest.json` to determine which repos need attention without re-running expensive operations.

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
