# Dusk Ocean CLI
Dusk Ocean CLI tool is used to manage polyglot mono-repos. 

## Getting Started
To get started, install the CLI from {{TBD}}, then run:
```bash
    ocean init --name <workspace_name> --registry <registry_address>
```
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
    "version": "0.1.0",
    "docker_registry": "localhost:3000",
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
- Language and `type` live in each repo's `ocean.config.json` (service|library|project).
- Global libs/projects must have unique names (use `-ts/-py/-go` suffix when names collide).

#### Repositories
Each repository (service, library, or project) contains an ocean.config.json at its root. This allows Dusk Ocean to remain language-agnostic and exposes `language` and `type` metadata.

#### Migration
Use `scripts/migrate-language-folders.py` to move language folders to the new flat layout. Run with `--dry-run` first, then re-run without it.

```json
{
    "name": "my-cool-library",
    "language": "typescript",
    "type": "library",
    "tasks": {
        "build": "npm run build",
        "test": "npm run test",
        "install": "npm install @dusk/library_name"
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
- `ocean-workspace.json` (if missing) with workspace name, registry, and starter app/service placeholders.
- `.ocean/` with `results/` and `hashes/`.
- `repos/` with `apps/`, `libs/`, `containers/`, and `templates/`.
- `repos/templates/apps/` with subfolders `services/`, `libs/`, `clients/`, `jobs/`, and `testing/`.
- `repos/templates/apps/docker-compose.yml` and `repos/templates/apps/docker-compose.dev.yml`.
- `.gitkeep` files in newly created directories.
- Updates `.gitignore` to include `.ocean` (creates `.gitignore` if needed; avoids duplicates).

#### Add
Scaffold new components without manual boilerplate setup.
- `ocean add app --name <name>`: Creates the folder structure and basic docker-compose files (including Redis and HashiCorp Vault).
- `ocean add service`: Prompts for an app, name, template, and database attachment choice, then scaffolds the service. The flow and validation are:
  - App selection: choose from existing apps; errors if no apps exist.
  - Service name: required, no spaces, and letters only (A-Z/a-z).
  - Template selection: choose from available API templates or "none (boilerplate)"; errors if no templates exist.
  - Attach database: yes/no prompt (selection is captured, but no side effects are currently applied).
  - Template placeholders: if the selected template contains `{{placeholders}}` in file names or contents, prompts for each value and requires non-empty input.
  - Applies scaffold output, assigns ports, updates `ocean-workspace.json`, and wires the service into the app's `docker-compose.yml` and `docker-compose.dev.yml`.
  - `ocean add library`: Prompts for location (Global or App-specific) and template. For Go libraries, it automatically runs go work use.
  - `ocean add project`: Scaffolds an external-facing project in the repos/projects directory from the selected template.

#### Run
Manage local development environments.
- `ocean run app`: Merges base and dev Compose files to spin up a full environment. Use `--no-dev` for a production-like local run.
- `ocean run service`: Allows interactive selection of one or more services to run.

#### Build & Check
- `ocean build (project|service|library) --name <name>`: Executes the build task defined in the config if the hash has changed.
- `ocean check (project|service|library) --name <name>`: Executes the test suite and outputs JUnit XML to .ocean/results.

#### Install
Install a local dependency into the current target directory using the dependency's install task.
- `ocean install <dependency>`: Run from inside a service, app library, global library, or project folder.
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
project           -> global library
```

#### Contain & Refresh
- `ocean contain service --name <name>`: Builds and publishes a Docker image to the configured registry.
- `ocean refresh`: Performs a state cleanup. It validates hashes, ensures port consistency, and checks image names across orchestration configs.
- `ocean refresh --clear-hashes`: Removes all build/check hash files when test runs appear stuck or need a clean rebuild.

#### Detach
Detaching allows code to be removed from the monorepo for delivery to clients or external hosting.
- `ocean detach app`: Safely extracts a full application and its local dependencies.
- `ocean detach package`: Extracts a single library or project.

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
    "python.testing.pytestPath": "ocean", 
    "python.testing.pytestArgs": [
        "check", "service", 
        "--name", "${workspaceFolderBasename}", 
        "--internal-runner"
    ]
}
```

- Discovery: The Python extension will still scan your files to find `test_` functions.
- Execution: when you click "Play", it calls `ocean check`, which spins up the `docker-compose + docker-compose.dev` stack, runs the tests, and shuts it down.

##### Typescript
For Typescript, we recommend using Jest with a custom command prefix. However, both are possible:

###### Jest
To use Jest, you should use the Jest Extension by Orta. Since this extension looks for the jest binary, you must configure it to call the CLI instead. In `.vscode/settings.json`:
```json
{
    "jest.jestCommandLine": "ocean check",
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
    "vitest.commandLine": "ocean check library --name my-ui-lib --"
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
