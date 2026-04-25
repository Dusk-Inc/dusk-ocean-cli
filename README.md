![dusk ocean image header](./resources/Dusk%20Ocean%20Github%20Readme.png "dusk ocean image header")

# Dusk Ocean CLI
Dusk Ocean is a polyglot polyrepo CLI for scaffolding repositories, managing dependency wiring, and executing cached build/check workflows. Most commands are fully flag-driven and composable in scripts or agent workflows.

## Source Layout
- `src/commands`: Cobra command definitions — one file per top-level command.
- `src/functions`: All business logic, pure helpers, and domain operations.
- `src/interfaces`: Side-effect boundaries and adapters (e.g. command runners).
- `src/models`: Shared data shapes used across the codebase.
- `src/tokens`: Shared constants and enums for domain literals and kinds.

## Getting Started
Run the following from the root of your polyrepo to initialize the workspace:
```bash
dusk-ocean init --name <workspace_name>
```
All CLI commands must be prefixed with `dusk-ocean`.


Testing

`init` creates:
- `ocean.workspace.json` — workspace registry (apps, libraries, projects, templates, ports).
- `.ocean/` with `results/` and `hashes/` subfolders.
- `repos/` with `apps/`, `libs/`, `projects/`, `templates/`, and `containers/` subdirectories. Each registered template lives in its own folder under `repos/templates/<name>/`.
- `.gitkeep` files in newly created empty directories (except `.ocean/` folders).
- A `.gitignore` entry for `.ocean` (created or updated; no duplicates added).

Note: Apps are not template-able. `dusk-ocean menu create` (app type) scaffolds the app folder structure directly in code, so `init` does not create a `repos/templates/apps/` tree.

## Core Concepts

### Organization
All code lives under `repos/` in one of four categories:

#### Apps (`repos/apps/<app>/`)
Full-stack or microservice-based applications. Apps are not template-able — `dusk-ocean menu create` scaffolds the folder structure directly in code. Each app contains:
- `services/` — API and backend service logic.
- `libs/` — Libraries internal to that specific application.
- `testing/` — App-scoped integration or system testing projects (polyglot).
- `jobs/` — Orchestration, deployment, and CI/CD configurations.
- `docs/` — App-scoped documentation.

#### Global Libraries (`repos/libs/<name>`)
Libraries shared across the entire monorepo. Use a `-ts`, `-py`, or `-go` suffix when names collide across languages.

#### Projects (`repos/projects/<name>`)
Standalone repositories for self-contained tools, CLIs, research repos, or other non-library, non-application work. Projects participate in the standard build/check/install workflow and may depend on global libraries, but cannot be used as dependencies by other repositories. Use a language suffix when names collide.

#### Templates (`repos/templates/<name>/`)
Scaffolding sources used by `menu create` to stamp out new services, libraries, or projects. **Apps are intentionally not template-able** — Dusk Ocean scaffolds the app folder structure directly in code. Each template is its own repo with an `ocean.config.json` and is registered in `ocean.workspace.json` with a `kind` field declaring what it produces (`service`, `library`, or `project`). A template may also declare a `deps` list of local libraries; those dependencies are automatically wired into anything scaffolded from the template (see [`menu create`](#menu)). Templates participate in the workspace registry but are **excluded from** `build`, `check`, `install`, `run`, `contain`, `refresh`, and the hash manifest — they are not buildable artifacts. Templates cannot themselves be used as dependencies by other repositories.

### Workspace Configuration (`ocean.workspace.json`)
Tracks all registered apps, services, libraries, projects, and templates along with their local dependency graphs.

```json
{
    "workspace": "workspace-name",
    "variables": {
        "org": "dusk-inc"
    },
    "tasks": {
        "clone":             "git clone {{repo:remote}} {{repo:path}}",
        "init":              "git init {{repo:path}}",
        "create_remote":     "gh repo create {{var:org}}/{{repo:name}} --private --source {{repo:path}} --remote origin",
        "checkout_existing": "git checkout {{repo:branch}}",
        "checkout_new":      "git checkout -b {{repo:branch}}"
    },
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
                    "remote": "git@github.com:dusk-inc/svc-a.git",
                    "variables": {
                        "branch": "main"
                    },
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
    ],
    "templates": [
        {
            "name": "ts-service-starter",
            "kind": "service",
            "deps": [
                { "lib": "lib-a", "from": "global" }
            ]
        }
    ]
}
```

Each `deps` entry specifies the library name (`lib`) and where it comes from (`from`). The `from` value is either `"global"` for workspace-level libraries or the app name for app-scoped libraries. Projects and templates may depend on global libraries but cannot themselves be used as dependencies. A template's `kind` field declares what it scaffolds (`service`, `library`, or `project` — apps are not template-able); its `deps` list describes libraries that the **scaffolded repo** receives at creation time, not libraries the template itself consumes.

### Repository Configuration (`ocean.config.json`)
Every app, service, library, project, and template has an `ocean.config.json` at its root. This is the language-agnostic task definition file. (Templates only need a minimal config — most task fields stay empty since templates are not built or run.)

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
        "publish": "",
        "run": ""
    }
}
```

- `build` and `test`: invoked by `dusk-ocean build` and `dusk-ocean check`.
- `install`: invoked by `dusk-ocean install` to run the package manager (e.g. `pnpm install`).
- `add` and `uninstall`: invoked when wiring or unwiring this library as a local dependency in another repo.
- `contain`: invoked by `dusk-ocean contain` to build and publish a service or project container image.
- `publish`: invoked by `dusk-ocean publish` to publish a project artifact to a package registry (e.g. `npm publish`).
- `run`: invoked by `dusk-ocean run` to execute the app or service after pre-flight checks.

### Variables

Any task command — whether in `ocean.workspace.json` or in a repo's `ocean.config.json` — may reference values through four variable namespaces. Each namespace has a distinct prefix so there is no ambiguity at the site of use.

| Token form | Source | Scope |
|---|---|---|
| `{{env:NAME}}` | `.env` file at the workspace root | Loaded once; values are the same regardless of which repo a task runs against |
| `{{var:NAME}}` | `variables` map at the top of `ocean.workspace.json` | Workspace-global, user-defined |
| `{{ocean:NAME}}` | Built-in reserved tokens fixed by Dusk Ocean | System — cannot be declared by the user |
| `{{repo:NAME}}` | Fields on a specific repo entry in `ocean.workspace.json` | Re-evaluated per repo every time a task runs against a different repo entry |

Namespaces do not fall back to each other. A missing key in any namespace is a hard error at task-resolution time, not a silent empty string.

#### Reserved `{{ocean:*}}` tokens
These are supplied by Dusk Ocean at runtime and users cannot redefine them. The set currently in effect is the contain-time tokens documented under [`contain`](#contain): `{{ocean:service_name}}`, `{{ocean:port}}`, `{{ocean:image_path}}`, `{{ocean:container_file}}`. Future system tokens will also live under this prefix.

#### Reserved `{{repo:*}}` names
When a task runs against a specific repo entry, the following names resolve automatically from fields on that entry. Users **must not** declare any of these inside a repo's user-defined `variables` block; doing so is a validation error that names the offending repo and the colliding key.

- All repo kinds: `name`, `kind`, `path`, `scopes`, `remote`
- Services additionally: `port`, `image_name`, `image_tag`, `dockerfile`, `container_file`, `image_path`, `app`
- App-scoped libraries additionally: `app`

`{{repo:remote}}` resolves to the repo's `remote` field (the git URL — see [Polyrepo & Remotes](#polyrepo--remotes) for the full shape).

#### Per-repo scope semantics
A workspace-level task command is a **template**. When Dusk Ocean runs that template against a specific repo entry, `{{repo:*}}` tokens resolve against that entry. The same template therefore produces a different concrete command per repo.

Given the workspace `clone` task:

```
git clone {{repo:remote}} {{repo:path}}
```

and two repo entries each carrying their own reserved fields:

```jsonc
// service svc-a
{ "name": "svc-a", "remote": "git@github.com:dusk-inc/svc-a.git" }  // path → repos/apps/app-a/services/svc-a
// library lib-a
{ "name": "lib-a", "remote": "git@github.com:dusk-inc/lib-a.git" }  // path → repos/libs/lib-a
```

the same `clone` template expands to:

```
git clone git@github.com:dusk-inc/svc-a.git repos/apps/app-a/services/svc-a
git clone git@github.com:dusk-inc/lib-a.git repos/libs/lib-a
```

#### Defining user-supplied variables
Workspace-global user variables live in a top-level `variables` map on `ocean.workspace.json`:

```json
{
    "workspace": "workspace-name",
    "variables": {
        "org": "dusk-inc"
    }
}
```

Referenced as `{{var:org}}`.

Per-repo user variables live in a `variables` block on any service, library, or project entry. Use this for anything that isn't already a reserved `{{repo:*}}` name:

```json
{
    "name": "svc-a",
    "remote": "git@github.com:dusk-inc/svc-a.git",
    "variables": {
        "branch": "main",
        "deploy_env": "staging"
    }
}
```

Referenced as `{{repo:branch}}`, `{{repo:deploy_env}}`. These coexist with the reserved `{{repo:*}}` names above; declaring a user variable with the same name as a reserved one (e.g. `name`, `remote`, `path`) is a validation error.

### Workspace Tasks

Workspace tasks are command templates declared at the top level of `ocean.workspace.json` and invoked by Dusk Ocean against a single repo entry via `dusk-ocean task --name <task> --target <repo> [--app <app>]`. Templates freely mix `{{var:*}}` (workspace-global), `{{env:*}}` (workspace `.env`), and `{{repo:*}}` (per-repo) tokens — the `{{repo:*}}` portion resolves against the target repo. Iteration across multiple repos is intentionally not supported yet.

```json
{
    "workspace": "workspace-name",
    "variables": { "org": "dusk-inc" },
    "tasks": {
        "clone":             "git clone {{repo:remote}} {{repo:path}}",
        "init":              "git init {{repo:path}}",
        "create_remote":     "gh repo create {{var:org}}/{{repo:name}} --private --source {{repo:path}} --remote origin",
        "checkout_existing": "git checkout {{repo:branch}}",
        "checkout_new":      "git checkout -b {{repo:branch}}"
    }
}
```

In the example above, `org` is a workspace constant defined in `variables`; `remote`, `path`, and `name` are reserved `{{repo:*}}` names auto-derived from the repo entry (see [Variables](#variables)); and `branch` is the only **user-defined** `{{repo:*}}` value — each repo entry that participates in `checkout_existing` / `checkout_new` must supply it in its own `variables` block.

### Polyrepo & Remotes

Dusk Ocean is a polyrepo workspace manager. Each service, library, project, and template registered in `ocean.workspace.json` may live in its own git repository, and Dusk Ocean stores the upstream location on the repo entry itself so workspace-level tasks (see [Workspace Tasks](#workspace-tasks)) can clone or publish each one without the developer hand-maintaining a parallel URL list.

#### The `remote` field
Each repo entry (service, library, project, or template) may carry a `remote` attribute sibling to `name`, `deps`, etc. It is a plain string holding the git URL and is exposed to templates as the reserved `{{repo:remote}}` token.

```json
{
    "name": "svc-a",
    "remote": "git@github.com:dusk-inc/svc-a.git",
    "deps": []
}
```

No branch, auth, or VCS metadata is tracked alongside the URL. If cloning requires a token or secret, resolve it from an environment variable inside the task command itself — for example `{{env:GITHUB_TOKEN}}` in a workspace `clone` task. Dusk Ocean has no knowledge of secrets or how they are used; they are just variable values.

When the upstream URL is unknown or not yet decided, `remote` may be set to the literal string `"None"`. The developer can edit it later by hand. A repo with `remote: "None"` is still a first-class workspace member; it just can't participate in tasks that actually need the URL (e.g. the workspace `clone` task).

#### Registration marker
The presence of an `ocean.config.json` at the root of a repo directory is Dusk Ocean's registration marker. `adopt` and `register` both use it as the signal for "this repo is under Dusk Ocean management" and refuse to touch a directory that already has one.

#### `adopt` — clone an external repo into the workspace
Use `adopt` when the repo lives on a git host but isn't on disk yet. It clones the repo into the deterministic workspace path, registers it in `ocean.workspace.json`, and bootstraps a starter `ocean.config.json`.

```bash
dusk-ocean adopt <remote-url> --kind <kind> --name <name> [--app <app>] [--template-kind <kind>]
```

On invocation, `adopt` checks the deterministic target path `repos/<...>/<name>/` chosen by `--kind` and behaves as follows:

| Target path state | Behavior |
|---|---|
| Does not exist | Clone `<remote-url>` into it, write a starter `ocean.config.json` at the root, register the new entry in `ocean.workspace.json` with `remote` populated from `<remote-url>`. |
| Exists, no `ocean.config.json` inside | Refuse with an error that suggests running `register` instead. |
| Exists, contains an `ocean.config.json` | Refuse with an error stating the repo is already registered. |

If `--name` is omitted, Dusk Ocean defaults it to the basename of the remote URL (`git@github.com:dusk-inc/svc-a.git` → `svc-a`). The folder layout under `repos/` is not configurable — Dusk Ocean relies on the deterministic path for every other workflow.

#### `register` — bring an already-on-disk repo into the workspace
Use `register` when a developer has already cloned (or otherwise placed) a repo at one of the allowed workspace locations and wants Dusk Ocean to start managing it. `register` does **not** clone; it only bootstraps the config at the repo root and records the entry in `ocean.workspace.json`.

```bash
dusk-ocean register --kind <kind> --name <name> [--app <app>] [--remote <url>] [--template-kind <kind>]
```

On invocation, `register` checks the deterministic target path for `--kind` / `--name` (and `--app`, where required) and behaves as follows:

| Target path state | Behavior |
|---|---|
| Does not exist | Refuse with a not-found error. |
| Exists, no `ocean.config.json` inside | Write a starter `ocean.config.json` at the root, register the new entry in `ocean.workspace.json` with `remote` set from `--remote`, or to the literal string `"None"` if the flag was omitted. |
| Exists, contains an `ocean.config.json` | Refuse with an error stating the repo is already registered. |

Both commands are also available through `dusk-ocean menu`, which prompts for `--kind`, `--name`, parent app (when applicable), and the remote URL.

#### `--kind` values
The `--kind` flag (shared by `adopt` and `register`) selects where the repo is registered in `ocean.workspace.json` and therefore where on disk it lives. The choices mirror the existing workspace taxonomy:

| `--kind` value | Registered under | Filesystem location |
|---|---|---|
| `project` | top-level `projects` list | `repos/projects/<name>/` |
| `library` (no `--app`) | top-level `libraries` list | `repos/libs/<name>/` |
| `library` + `--app <app>` | `apps[<app>].libraries` | `repos/apps/<app>/libs/<name>/` |
| `app` | top-level `apps` list (as a full application repo) | `repos/apps/<name>/` |
| `service` | `apps[<app>].services` | `repos/apps/<app>/services/<name>/` |
| `template` | top-level `templates` list | `repos/templates/<name>/` |

`--app` is **required** for `service`, **optional** for `library` (presence flips the entry from global to app-scoped), and **rejected** for `project`, `app`, and `template`. Registering a template additionally requires `--template-kind <service|library|project>` so the entry knows what it scaffolds. **Apps are not template-able**: `--template-kind app` is explicitly rejected.

#### The auto-generated starter `ocean.config.json`
Both `adopt` and `register` drop the same starter file at the repo root. It is intentionally minimal — every task starts empty, and the developer fills each one in by delegating to whatever build system the repo already ships with (npm scripts, a Makefile, a Taskfile, cargo, go test, etc.).

```json
{
    "name": "svc-a",
    "language": "",
    "type": "service",
    "tasks": {
        "build":     "",
        "test":      "",
        "install":   "",
        "add":       "",
        "uninstall": "",
        "contain":   "",
        "run":       ""
    }
}
```

Typical delegations a developer would drop in:

- `"build": "npm run build"` — reuse an existing `package.json` script.
- `"test": "make test"` — delegate to a Makefile target.
- `"install": "go mod download"` — call the language's native tool directly.
- `"contain": "docker build -f {{ocean:container_file}} -t {{ocean:image_path}} ."` — mix delegation with Dusk Ocean's reserved tokens.

Because the tasks are plain shell commands, there is no lock-in: Dusk Ocean never assumes a specific build tool. It only needs a command it can run in the repo's directory.

### Build, Check, Contain, and Publish Caching
To avoid redundant work, Dusk Ocean computes a dependency-tree hash for each repository and compares it to the stored per-operation hash in `.ocean/manifest.json`:
- If the dependency-tree hash matches the stored `build_hash`, `check_hash`, `contain_hash`, or `publish_hash`, the corresponding operation is skipped.
- If the hash differs (or is missing), the operation executes and the new hash is saved on success.
- For `check`, if the hash changed and a `build` task exists, Ocean rebuilds first, then runs the check.
- The `contain` and `publish` operations use the same hash-based skip logic applied to the target's dependency tree.

## Commands

### `init`
Initialize a new workspace.
```bash
dusk-ocean init --name <workspace_name>
```

### `menu`
Guided, interactive interface for all CLI commands. Each menu entry describes the command and collects required inputs via prompts. Scaffolding (`menu create`) and repo deletion (`menu remove`) are exclusively available through the menu and have no flag-based equivalents. All other commands (including `run`, `rename`, `add`, `remove`, etc.) are accessible both through the menu and directly via flags.
```bash
dusk-ocean menu         # top-level command selector
dusk-ocean menu create  # scaffold a new app, service, or library (prompts for type, name, template)
dusk-ocean menu remove  # delete a repo and clean up all references (prompts for confirmation)
```

`menu create` scaffolds:
- **App**: directly creates `repos/apps/<name>/` with `services/`, `jobs/`, `libs/`, `docs/`, and `testing/` subfolders and registers it in workspace config. Apps are not template-driven — the layout is hard-coded in the CLI.
- **Service**: prompts for the parent app, the service name, an optional template, and how to assign a container file (existing file from `repos/containers/`, custom path, or none). The selection is recorded as the service's `container_file` in workspace config.
- **Library**: prompts for workspace-level (`repos/libs/<name>/`) or app-adjacent (`repos/apps/<app>/libs/<name>/`) placement, scaffolds from the selected template, and registers it in workspace config.
- **Project**: prompts for the project name and a template, then scaffolds `repos/projects/<name>/` and registers it.

Templates registered in `ocean.workspace.json` drive `menu create`'s template list (filtered by each template's `kind`). When the user picks a template, Dusk Ocean copies the template directory to the new repo's path, registers the new repo in workspace config, and then walks the template's `deps` and wires each one into the freshly-created repo using the same flow as [`dusk-ocean add`](#add--dependency-wiring) — running each dependency's `add` task in the new repo's directory and recording the relationship in the workspace dependency graph. Standard scope, cycle, and flow validation apply: if any propagated dep would be illegal for the target kind, the scaffold fails **before** any files are copied so workspace state stays clean.

`menu remove` deletes the selected repo directory, runs uninstall tasks in all dependents if it is a library, and removes all references from workspace config.

Template files containing `{{placeholder}}` tokens will trigger prompts for replacement values applied to both file names and file content. Tokens prefixed with `ocean:` (e.g. `{{ocean:port}}`) are reserved system placeholders: they are **not** prompted at scaffold time and survive verbatim in the generated files for runtime substitution by Dusk Ocean.

### `add` — Dependency Wiring
Wire a local dependency into another repository. This runs the library's `add` task inside the target directory and registers the relationship in workspace config.
```bash
dusk-ocean add --payload <lib-name> --target <repo-name>
```
Rules:
- A repo cannot depend on itself.
- Projects cannot be used as dependencies by apps, services, or libraries.
- Templates cannot be used as dependencies and cannot be the `--target` of `dusk-ocean add`. Their `deps` are propagated only at scaffold time via `menu create`.
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
Skips install or build for nodes that have no corresponding task. Templates are skipped entirely — they are scaffolding sources and have no buildable artifact. Fails on dependency graph cycles. Cleans up stale hash files for repos no longer in workspace config.

### `contain`
Stage a minimal build context and execute the service's or project's `contain` task to build and publish a container image.
```bash
dusk-ocean contain project --name <name>
dusk-ocean contain service --name <name>
dusk-ocean contain service --name <name> --in <app>   # required when service name is ambiguous
```

Rather than enforcing a specific containerization tool, Dusk Ocean executes the `contain` task (defined in `ocean.config.json`) after staging the build context. Before executing, Dusk Ocean substitutes reserved placeholders (`{{ocean:service_name}}`, `{{ocean:port}}`, `{{ocean:image_path}}`, `{{ocean:container_file}}`) in the task command with runtime values. Projects have no `port` or `image_path` fields in workspace config; for project targets, those two tokens are substituted with empty strings, and `{{ocean:container_file}}` falls back to `<staging>/repos/projects/<name>/Dockerfile`. See [Variables](#variables) for the full substitution model. Ocean copies the target directory and all of its transitive local dependencies into `.ocean/stage/`, preserving their paths relative to the workspace root. The staging directory is always removed after the contain task completes or fails.

Each service may declare `container_file` (path to the container build recipe) and `image_path` (full registry path for the built image) in workspace config. A bare filename for `container_file` resolves to `repos/containers/<name>`; a value with path separators is treated as workspace-root-relative.

Dusk Ocean computes a dependency-tree hash before contain; if the hash matches the stored `contain_hash` in the manifest, the build is skipped.

Two workspace-root files control staging:
- `.oceanignore`: gitignore-format patterns for files to exclude (e.g. `node_modules/`). Absence is logged; no patterns are applied.
- `.oceaninclude`: one workspace-relative path per line; files are copied to the staging root (e.g. `go.work`, `pnpm-workspace.yaml`). Absence is logged; no files are copied.

### `publish`
Execute a project's `publish` task (e.g. `npm publish`). Unlike `contain`, publish runs from the actual project directory — no staging — because package managers rely on the real repo layout (`package.json`, `.npmignore`, etc.).
```bash
dusk-ocean publish project --name <name>
dusk-ocean publish project --name <name> --skip-preflight
```
Pre-flight requires a prior successful `build` and `contain` for the target (`build_hash` and `contain_hash` must be present in the manifest). This prevents publishing a stale artifact. Use `--skip-preflight` to bypass for emergency releases.

Dusk Ocean computes a dependency-tree hash before publish; if the hash matches the stored `publish_hash` in the manifest, the publish is skipped.

The `publish` task inherits the CI environment, so secrets (e.g. `NODE_AUTH_TOKEN`) flow through from the caller.

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

Templates are not hashed and never appear in `.ocean/manifest.json` — they have no buildable output to track staleness for.

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
