# Dusk Ocean CLI
Dusk Ocean CLI tool is used to manage polyglot mono-repos. 

## Source Layout
- `src/modules`: feature-level orchestration and command flows.
- `src/functions`: reusable pure helpers (for example dependency key builders).
- `src/interfaces`: side-effect boundaries and adapters (for example command runners).
- `src/models`: shared data shapes used across modules.
- `src/tokens`: shared constants/enums for domain literals and kinds.

## Getting Started
To get started, install the CLI from {{TBD}}, then run:
```bash
    dusk-ocean init --name <workspace_name>
```
All CLI commands must use the `dusk-ocean` prefix (`ocean` is not supported).
This command initializes the workspace by creating the ocean-workspace.json configuration file and a .ocean metadata folder. It also ensures a .gitignore exists and adds .ocean to it.
It also scaffolds the base workspace directory structure and template files:
- `.ocean/` with `results/` and `hashes/` subfolders.
- `repos/` with `apps/`, `libs/`, `containers/`, and `templates/`.
- `repos/templates/apps/` plus app template subfolders (`services/`, `libs/`, `clients/`, `jobs/`, `testing/`).
- Default template files in `repos/templates/apps/`: `docker-compose.yml` and `docker-compose.dev.yml`.
- Adds `.gitkeep` files for newly created directories so empty folders are tracked.
- Ensures `.gitignore` contains a single `.ocean` entry (creates the file if missing).
- Creates at the root of the repo a `ocean.workspace.json` file to configure the workspace.

## Core Concepts
### Organization
Dusk Ocean organizes all code into a structured repos folder to maintain consistency across the monorepo.

#### Apps
The apps folder contains full-stack or microservice-based applications. Each app is subdivided into:
- Services: API and backend service logic.
- Libs: Libraries internal to that specific application.
- Testing: App-scoped integration/system testing projects (polyglot).
- Jobs: Orchestration, deployment, and CI/CD configurations.

#### Libs
Global libraries meant for internal use across the entire monorepo. They live under `repos/libs/<name>` (use a `-ts/-py/-go` suffix when names collide across languages). Language is stored in each repo's `ocean.config.json`.

#### Projects
Codebases intended for open-source distribution or external consumption. They live under `repos/projects/<name>` (use a `-ts/-py/-go` suffix when names collide across languages). Language is stored in each repo's `ocean.config.json`.

### Configuring
#### Workspace
The workspace has a single `ocean.workspace.json` file that configures the mono-repo's:
- dependencies
- ports
- addresses
- image names and tags

Which looks like this:

```json
{
    "workspace": "workspace-name",
    "ports": {
        "allowed": {
            "min": 3000,
            "max": 3999
        },
        "reserved": [
            {
                "name": "Codex",
                "port": 1455
            }
        ]
    },
    "apps": [
        {
            "name": "app-1",
            "services": [
                {
                    "name": "service-1",
                    "port": 3000,
                    "image": {
                        "name": "app-1__service-1",
                        "tag": "dev"
                    },
                    "Dockerfile": "ts.Dockerfile"
                }
            ],
            "libraries": [
                {
                    "name": "app-library-1",
                    "deps": [
                        "library-1"
                    ]
                },
            ]
        }
    ],
    "libraries": [
        {
            "name": "library-1",
            "deps": [
                "library-2"
            ]
        },
        {
            "name": "library-2",
            "deps": []
        },
        {
            "name": "library-3",
            "deps": []
        }
    ],
    "projects": [
        {
            "name": "project-1",
            "deps": [
                "library-2"
            ]
        }
    ]
}
```

Service images include a name and tag. The name defaults to `app-name__service_name`, and the default tag is `dev`.

Language requirements:
- Language and `type` live in each repo's `ocean.config.json` (service|library|project|test).
- Global libs/projects must have unique names (use `-ts/-py/-go` suffix when names collide).

#### Repositories
Each repository (service, library, or project) contains an ocean.config.json at its root. This allows Dusk Ocean to remain language-agnostic and exposes `language` and `type` metadata. Libraries should define `tasks.add` and `tasks.uninstall` for dependency wiring alongside `tasks.install` for local dependency installs.
```json
{
    "name": "service-a",
    "language": "typescript",
    "type": "service",
    "tasks": {
        "build": "pnpm build",
        "test": "pnpm test",
        "install": "pnpm install"
    }
}
```

#### Building and Testing
To optimize performance, Dusk Ocean uses a hashing system:
1. Hashing: When a build or check command is run, Ocean generates a hash of the source code.
2. Comparison: The hash is compared against the value stored in .ocean/hashes.
3. Execution:
- If the hash matches the previous run, the task is skipped, and the previous results are preserved.
- If the hash is different, the command executes and the new hash is saved.
4. Test/Build Dependency: If a test is triggered and the code has changed, Ocean will automatically trigger a rebuild if the build hash is also outdated.

### Commands
Dusk Ocean makes it easy to create and extend existing applications. Here is an overview of the commands:

#### Init
Initializes the workspace and creates the ocean-workspace.json. This file tracks:
- Global workspace settings.
- Allowed and reserved ports for services.
- A registry of all apps and their constituent services (including ports and image names).
It also creates the base workspace structure and template files:
- `ocean.workspace.json` (if missing) with workspace name and starter app/service placeholders.
- `.ocean/` with `results/` and `hashes/`.
- `repos/` with `apps/`, `libs/`, `containers/`, and `templates/`.
- `repos/templates/apps/` with subfolders `services/`, `libs/`, `jobs/`, and `testing/`.
- `repos/templates/apps/docker-compose.yml` and `repos/templates/apps/docker-compose.dev.yml`.
- `.gitkeep` files in newly created directories.
- Updates `.gitignore` to include `.ocean` (creates `.gitignore` if needed; avoids duplicates).

#### Add
Scaffold new components without manual boilerplate setup.
- `dusk-ocean add app --name <name>`: Creates the folder structure and basic docker-compose files (including Redis and HashiCorp Vault).
- `dusk-ocean add service`: Prompts for an app, name, template, and database attachment choice, then scaffolds the service. The flow and validation are:
  - App selection: choose from existing apps; errors if no apps exist.
  - Service name: required, no spaces, and letters only (A-Z/a-z).
  - Template selection: choose from available API templates or "none (boilerplate)"; errors if no templates exist.
  - Attach database: yes/no prompt (selection is captured, but no side effects are currently applied).
  - Template placeholders: if the selected template contains `{{placeholders}}` in file names or contents, prompts for each value and requires non-empty input.
  - Applies scaffold output, assigns ports, updates `ocean-workspace.json`, and wires the service into the app's `docker-compose.yml` and `docker-compose.dev.yml`.
  - `dusk-ocean add library`: Prompts for location (Global or App-specific) and template. Then, the CLI adds an entry to ocean.workspace.json in the "libraries" array.
  - `dusk-ocean add project`: Scaffolds an external-facing project in the repos/projects directory from the selected template. Then, the CLI adds an entry to ocean.workspace.json in the "projects" array.
  - `dusk-ocean add test`: Prompts for app, name, and test template, then scaffolds to `repos/apps/<app>/testing/<name>` and tracks it in workspace config.

#### Run
Manage local development environments.
- `dusk-ocean run app`: Merges base and dev Compose files to spin up a full environment. Use `--no-dev` for a production-like local run.
- `dusk-ocean run service`: Allows interactive selection of one or more services to run.

#### Build & Check
- `dusk-ocean build (project|service|library|test) --name <name>`: Executes the build task defined in the config if the hash has changed.
- `dusk-ocean check (project|service|library|test) --name <name>`: Executes the test suite and outputs JUnit XML to .ocean/results.

#### Install
Install a local dependency into the current target directory using the dependency's add task.
- `dusk-ocean install <dependency>`: Run from inside a service, app library, global library, or project folder.
Allowed dependency flows:
```
Target            -> Dependency
global library    -> global library
app library       -> app library (same app)
app library       -> global library
app library       -> project
service           -> global library
service           -> app library (same app)
service           -> project
app test          -> global library
app test          -> app library (same app)
app test          -> project
project           -> global library
```

#### Uninstall
Remove a local dependency from the current target directory using the dependency's uninstall task.
- `dusk-ocean uninstall`: Run from the workspace root and select the target and dependency to remove.

#### Contain & Refresh
- `dusk-ocean contain service --name <name>`: Builds and publishes a Docker image using the local Docker setup.
- `dusk-ocean refresh`: Installs dependencies, builds, and tests the full workspace dependency graph.
- `dusk-ocean refresh --clear-hashes`: Clears build/check hashes before running the refresh flow.

#### Detach
Detaching allows code to be removed from the monorepo for delivery to clients or external hosting.
- `dusk-ocean detach app`: Safely extracts a full application and its local dependencies.
- `dusk-ocean detach package`: Extracts a single library or project.

#### Remove
- `dusk-ocean remove app --name <name>`: Removes an app and deletes its workspace entries.
- `dusk-ocean remove library --name <name>`: Removes a library (use `--in <app>` for app libraries). Runs the library's `uninstall` task in every dependent service/library/project/test and removes the dependency entries from ocean.workspace.json.
- `dusk-ocean remove project --name <name>`: Removes a project and deletes its workspace entry.
- `dusk-ocean remove service --name <name> --in <app>`: Removes a service from the app and deletes its workspace entry.
- `dusk-ocean remove test --name <name> --in <app>`: Removes a testing project from an app and deletes its workspace entry.

### Orchestration
Dusk Ocean uses a layered Merge Strategy for Docker Compose to keep environments consistent:
- `docker-compose.yml`: The "Source of Truth" defining minimal services.
- `docker-compose.dev.yml`: Adds resource limits (CPU/Memory) and dev-specific overrides.
- `docker-compose.hashi.yml`: Injects Nomad, Consul, and Vault logic for high-fidelity orchestration.

## Integrations
### VS Code
Dusk Ocean acts as a bridge between the VS Code Test Explorer and your containers.

#### Standardized Handoff
Ocean post-processes results from any language into JUnit XML and stores them in .ocean/results/. This allows VS Code to visualize test failures and jump directly to the source code line, even if the test ran inside a Docker container.

#### Configurations
##### Pytest
To make the Pythen Test Explorer use Dusk Ocean, update your `.vscode/settings.json`.
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

- Discovery: The Python extension will still scan your files to find `test_` functions.
- Execution: when you click "Play", it calls `dusk-ocean check`, which spins up the `docker-compose + docker-compose.dev` stack, runs the tests, and shuts it down.

##### Typescript
For Typescript, we recommend using Jest with a custom command prefix. However, both are possible:

###### Jest
To use Jest, you should use the Jest Extension by Orta. Since this extension looks for the jest binary, you must configure it to call the CLI instead. In `.vscode/settings.json`:
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
    "jest.junitReporterOutputDir": ".ocean/results/${workspaceFolderBasename}"
}
```
Note for Jest: You must install jest-junit in your project. The CLI will intercept the command, check the hash, and then execute the underlying Jest task. If the hash hasn't changed, the CLI will simply signal the extension to read the existing XML in .ocean/results.

###### Vitest
If you prefer Vitest, the override is simpler. In `.vscode/settings.json`:
```json
{
    "vitest.commandLine": "dusk-ocean check library --name my-ui-lib --"
}
```

##### Go
Since Go Discovery is deeply tied to the `go` binary, use a wrapper or a VSCode task:
```json
{
    "go.alternateTools": {
      "go": "${workspaceFolder}/.ocean/bin/ocean-go-wrapper"
    }
}
```

##### Benefits
- Automatic Orchestration: you no longer need to manually run docker-compose up. The "play" button handles the lifecycle.
- Instant Feedback: if a test is skipped dur to an unchanged hash, the CLI instantly serves the last result from .ocean/results, and the sidebar turns green in milliseconds.
- Click-To-Source: because the CLI post-processes the XML, clicking a failure in the Test Explorer will jump your cursor directly to the failing lin in you `repos/` folder, even if the test run inside a container.
