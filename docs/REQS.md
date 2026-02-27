# Dusk Ocean Requirements
Dusk Ocean is a polyglot monorepo CLI for scaffolding repositories, managing dependency wiring, orchestrating local runtime flows, and executing cached build/check workflows.

## Requirements
### 1 Workspace Initialization
#### Context
`dusk-ocean init` bootstraps the workspace root and baseline repository layout.

#### Requirements
- [x] 1.1 Given `dusk-ocean init` is run with `--name`, when `ocean.workspace.json` does not exist, then Dusk Ocean shall create it with the provided workspace name, default allowed ports (`3000-3999`), and empty apps/libraries/projects lists.
- [x] 1.2 Given `dusk-ocean init` is run, when baseline directories are missing, then Dusk Ocean shall create `.ocean`, `.ocean/results`, `.ocean/hashes`, `repos`, `repos/apps`, `repos/libs`, `repos/projects`, `repos/containers`, and app-template directories under `repos/templates/apps`.
- [x] 1.3 Given new scaffold directories are created during init, when directory creation completes, then Dusk Ocean shall create `.gitkeep` files for tracked empty folders except `.ocean` folders.
- [x] 1.4 Given `repos/templates/apps/docker-compose.yml` or `repos/templates/apps/docker-compose.dev.yml` is missing, when init runs, then Dusk Ocean shall create empty template files.
- [x] 1.5 Given `.gitignore` is missing or does not contain `.ocean`, when init runs, then Dusk Ocean shall create/update `.gitignore` and ensure exactly one `.ocean` entry is present.
- [x] 1.6 Given `--name` is missing, when init is executed, then Dusk Ocean shall reject the command with a required-flag error.

#### Constraints
- Init shall be additive and not overwrite existing scaffold files by default.

### 2 Workspace Guard and Command Gating
#### Context
Most commands require execution from a valid workspace root.

#### Requirements
- [x] 2.1 Given any command except `init`, `version`, or `help` is executed, when the current directory is not the workspace root, then Dusk Ocean shall reject the command.
- [x] 2.2 Given guarded commands are executed, when `ocean.workspace.json` is missing at workspace root, then Dusk Ocean shall reject command execution.

#### Constraints
- Workspace root validation shall resolve absolute paths before comparison.

### 3 Scaffolding and Registration
#### Context
`add` commands scaffold repositories and register them in `ocean.workspace.json`.

#### Requirements
- [x] 3.1 Given `dusk-ocean add app --name <name>` is executed, when the app does not already exist, then Dusk Ocean shall copy the app template from `repos/templates/apps` into `repos/apps/<name>` and add the app to workspace config.
- [x] 3.2 Given `dusk-ocean add service` is executed, when app/template/dockerfile prompts complete successfully, then Dusk Ocean shall scaffold `repos/apps/<app>/services/<service>`, wire the service into `docker-compose.yml` and `docker-compose.dev.yml`, assign the next available allowed service port, and register the service image/dockerfile metadata in workspace config.
- [x] 3.3 Given `add service` runs with no template selection, when service scaffold is created, then Dusk Ocean shall generate `ocean.config.json` with service type and blank build/test tasks.
- [x] 3.4 Given `add library` is executed for global or app scope, when template copy succeeds, then Dusk Ocean shall register the new library in the appropriate workspace config location.
- [x] 3.5 Given `add project` is executed, when template copy succeeds, then Dusk Ocean shall scaffold `repos/projects/<name>` and register the project in workspace config.
- [x] 3.6 Given `add test` is executed, when template copy succeeds, then Dusk Ocean shall scaffold `repos/apps/<app>/testing/<name>` and register the test target in workspace config.
- [x] 3.7 Given template files include `{{placeholder}}` tokens, when scaffolding occurs, then Dusk Ocean shall prompt for replacement values and apply replacements to both file paths and file contents.

#### Constraints
- Service names shall be letters only.
- Library/project/test names shall reject whitespace and only allow configured character sets in command validation.

### 4 Local Runtime Orchestration
#### Context
`run` commands use Docker Compose files in app folders.

#### Requirements
- [x] 4.1 Given `dusk-ocean run app` is executed, when `--no-dev` is not set, then Dusk Ocean shall run `docker compose -f docker-compose.yml -f docker-compose.dev.yml up` from the app directory.
- [x] 4.2 Given `dusk-ocean run app --no-dev` is executed, when compose command is built, then Dusk Ocean shall exclude `docker-compose.dev.yml`.
- [x] 4.3 Given `dusk-ocean run service` is executed, when one or more services are selected and confirmed, then Dusk Ocean shall run docker compose `up` scoped to selected services.
- [x] 4.4 Given `run service` is executed for an app with zero services, when pre-run checks occur, then Dusk Ocean shall fail with a no-services error.

#### Constraints
- `run` commands shall stream subprocess stdout/stderr/stdin through the CLI.

### 5 Build and Check Execution with Dependency Order
#### Context
`build` and `check` commands execute repository tasks with dependency traversal and hash-based skipping.

#### Requirements
- [x] 5.1 Given a build target is selected (`app|service|library|project|test`), when dependencies are declared in workspace config, then Dusk Ocean shall build dependencies before the requested target using dependency order.
- [x] 5.2 Given a check target is selected (`app|service|library|project|test`), when dependencies are declared, then Dusk Ocean shall build dependencies before running target tests.
- [x] 5.3 Given a repository has no `build` task, when `build` executes, then Dusk Ocean shall skip that target and print a skip message.
- [x] 5.4 Given a repository has no `test` task, when `check` executes, then Dusk Ocean shall skip that target and print a skip message.
- [x] 5.5 Given directory hash equals prior hash for a build/check hash file, when command executes, then Dusk Ocean shall skip execution as unchanged.
- [x] 5.6 Given check hash changed and build task exists, when build hash is stale/missing, then Dusk Ocean shall run build before check for the same target.
- [x] 5.7 Given a `check` command completes (pass or fail), when command output is captured, then Dusk Ocean shall write JUnit-formatted results in `.ocean/results`.
- [x] 5.8 Given pass-through args are provided to `check`, when args are supplied after `--`, then Dusk Ocean shall append quoted pass-through args to the configured test command.

#### Constraints
- Pass-through args shall require `--` and shall reject positional args before separator.

### 6 Dependency Install and Uninstall Flow
#### Context
`install` and `uninstall` manage dependency entries and execute dependency-owned scripts.

#### Requirements
- [x] 6.1 Given `dusk-ocean install` is executed, when target/dependency prompts complete and flow validation passes, then Dusk Ocean shall run dependency `add` task in the target directory and persist dependency registration in workspace config.
- [x] 6.2 Given `install` is run from a repository cwd with a dependency name, when target resolution and validation succeed, then Dusk Ocean shall install without interactive target selection.
- [x] 6.3 Given dependency type is not allowed for target type, when install validation runs, then Dusk Ocean shall reject the install flow.
- [x] 6.4 Given install would create self-dependency, when registration validation runs, then Dusk Ocean shall reject the operation.
- [x] 6.5 Given `dusk-ocean uninstall` is executed, when target dependency is selected and confirmed, then Dusk Ocean shall run dependency `uninstall` task in the target directory and remove that dependency entry from workspace config.
- [x] 6.6 Given uninstall command is missing in dependency repo config, when uninstall execution is requested, then Dusk Ocean shall fail with a missing-uninstall-command error.

#### Constraints
- App-scoped libraries shall only be installable within the same app boundary.

### 7 Workspace Refresh
#### Context
`refresh` performs full dependency-ordered install/build/check across the workspace.

#### Requirements
- [x] 7.1 Given `dusk-ocean refresh` is executed, when workspace graph is non-empty, then Dusk Ocean shall run `install` task (if present), then build, then check for each node in dependency order.
- [x] 7.2 Given a node has no install task, when refresh install phase runs, then Dusk Ocean shall skip install for that node and print a skip message.
- [x] 7.3 Given `--clear-hashes` is set, when refresh begins, then Dusk Ocean shall remove build/check hash records before refresh execution.
- [x] 7.4 Given refresh preflight runs, when compose files have mismatched images/ports or duplicate ports across compose variants, then Dusk Ocean shall fail refresh with consistency errors.
- [x] 7.5 Given refresh completes, when stale hashes remain for removed targets, then Dusk Ocean shall clean stale hash files.

#### Constraints
- Refresh shall fail on dependency graph cycles.

### 8 Removal and Workspace Cleanup
#### Context
`remove` commands delete repos and clean workspace dependency references.

#### Requirements
- [x] 8.1 Given `remove app|service|library|project|test` is executed, when user confirmation is not `y`, then Dusk Ocean shall abort without mutation.
- [x] 8.2 Given `remove app|service|project|test` is confirmed, when target exists, then Dusk Ocean shall delete target directories and unregister target from workspace config.
- [x] 8.3 Given `remove library` is confirmed, when dependency targets reference that library, then Dusk Ocean shall run dependency uninstall tasks for dependents before removing the library and pruning dependency references from workspace config.
- [x] 8.4 Given requested remove target path does not exist, when remove executes, then Dusk Ocean shall return a target-not-found error.

#### Constraints
- Remove flows shall be interactive and confirmation-gated.

### 9 Container Publication
#### Context
`contain service` builds and publishes service container images.

#### Requirements
- [x] 9.1 Given `dusk-ocean contain service` is executed, when app/service are resolved from flags or prompts, then Dusk Ocean shall resolve image reference from workspace service image config.
- [x] 9.2 Given contain execution starts, when service path is resolved, then Dusk Ocean shall run `docker build -t <image> .` from `repos/apps/<app>/services/<service>`.
- [x] 9.3 Given local image build succeeds, when publication runs, then Dusk Ocean shall run `docker push <image>`.
- [ ] 9.4 Given service dependencies span app/global libraries, when contain builds service images, then Dusk Ocean shall support a minimal generated build context derived from `ocean.workspace.json` dependency closure instead of relying on a broad manual context.

#### Constraints
- Container build/push shall stream command output directly to CLI stdout/stderr.

### 10 Utility and Visibility Commands
#### Context
The CLI provides utility commands for version visibility and future detach flows.

#### Requirements
- [x] 10.1 Given `dusk-ocean version` is executed, when the command runs, then Dusk Ocean shall print the configured CLI version string.
- [x] 10.2 Given `dusk-ocean detach app` is executed, when current implementation runs, then Dusk Ocean shall return an explicit not-yet-implemented error.
- [x] 10.3 Given `dusk-ocean detach project` is executed, when current implementation runs, then Dusk Ocean shall return an explicit not-yet-implemented error.

#### Constraints
- Detach behavior is currently declared but not implemented.
