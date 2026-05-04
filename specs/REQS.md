# Dusk Ocean Requirements
Dusk Ocean is a polyglot monorepo CLI for scaffolding repositories, managing dependency wiring, and executing cached build/check workflows. Most commands are fully flag-driven and composable in scripts or agent workflows.

## Requirements
### 1 Workspace Initialization
#### Context
`dusk-ocean init` bootstraps the workspace root and baseline repository layout.

#### Requirements
- [x] 1.1 Given `dusk-ocean init` is run with `--name`, when `ocean.workspace.json` does not exist, then Dusk Ocean shall create it with the provided workspace name, default allowed ports (`3000-3999`), and empty apps and libraries lists.
- [x] 1.2 Given `dusk-ocean init` is run, when baseline directories are missing, then Dusk Ocean shall create `.ocean`, `.ocean/results`, `.ocean/hashes`, `repos`, `repos/apps`, `repos/libs`, `repos/projects`, `repos/templates`, and `repos/containers`. Apps are not template-able, so init shall not create any `repos/templates/apps/` tree.
- [x] 1.3 Given new scaffold directories are created during init, when directory creation completes, then Dusk Ocean shall create `.gitkeep` files for tracked empty folders except `.ocean` folders.
- [x] 1.4 Given `.gitignore` is missing or does not contain `.ocean`, when init runs, then Dusk Ocean shall create/update `.gitignore` and ensure exactly one `.ocean` entry is present.
- [x] 1.5 Given `--name` is missing, when init is executed, then Dusk Ocean shall reject the command with a required-flag error.

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

### 3 Menu: Prompt Interface
#### Context
`dusk-ocean menu` provides a guided, interactive interface for all CLI commands. Each menu entry describes the command and collects required inputs via prompts. Scaffolding (`menu create`) and repo deletion (`menu remove`) are exclusively available through the menu and have no flag-based equivalents. All other commands are accessible both through the menu and directly via flags.

#### Requirements
- [x] 3.1 Given `dusk-ocean menu` is executed, when the user selects a command, then Dusk Ocean shall present a description of the selected command and prompt for all required inputs before executing.
- [x] 3.2 Given `dusk-ocean menu create` selects app type, when prompts complete, then Dusk Ocean shall directly create `repos/apps/<name>/` with `services/`, `jobs/`, `libs/`, `docs/`, and `testing/` subdirectories (and a starter `ocean.config.json`) and register it in workspace config. Apps are not template-driven — these subfolders are created in code rather than copied from a template tree.
- [x] 3.3 Given `dusk-ocean menu create` selects library type, when the user selects workspace-level or app-adjacent placement and provides a name, then Dusk Ocean shall scaffold `repos/libs/<name>/` or `repos/apps/<app>/libs/<name>/` accordingly and register it in workspace config.
- [x] 3.4 Given template files include `{{placeholder}}` tokens, when scaffolding occurs, then Dusk Ocean shall prompt for replacement values and apply them to both file names and file contents.
- [x] 3.5 Given `dusk-ocean menu remove` is executed, when the user selects a target and confirms deletion, then Dusk Ocean shall delete the target directory and unregister it from workspace config.
- [x] 3.6 Given `menu remove` targets a library that has dependents, when deletion is confirmed, then Dusk Ocean shall run uninstall tasks for all dependent repos before removing the library and pruning dependency references from workspace config.
- [x] 3.7 Given user confirmation is not `y` in any `menu remove` flow, when the prompt resolves, then Dusk Ocean shall abort without mutation.

- [x] 3.8 Given `dusk-ocean menu create` selects service type, when the service scaffold prompt completes, then Dusk Ocean shall ask whether to assign a container file to the service: (a) select an existing file from `repos/containers/`, (b) provide a custom container file path, or (c) none; the selection shall be recorded as the service's `container_file` in workspace config.
- [x] 3.9 Given a scaffold template contains a placeholder prefixed with `ocean:` (e.g. `{{ocean:port}}`), when scaffolding occurs, then Dusk Ocean shall treat it as a reserved system placeholder, shall NOT prompt the user for a replacement value, and shall preserve it verbatim in the generated file for runtime substitution by Dusk Ocean.

#### Constraints
- Scaffolding (`menu create`) and repo deletion (`menu remove`) are only available through the menu and have no flag-based equivalents.
- All other commands available through the menu must also be fully executable via flags.
- Repository type names shall reject whitespace and only allow configured character sets.
- Reserved placeholder tokens (prefixed with `ocean:`) shall take precedence over user-prompted placeholders; a token matching both forms shall always be treated as reserved.

### 4 Build and Check Execution with Dependency Order
#### Context
`build` and `check` commands execute repository tasks with dependency traversal and hash-based skipping.

#### Requirements
- [x] 4.1 Given a build target is selected (`app|service|library|test`), when dependencies are declared in workspace config, then Dusk Ocean shall build dependencies before the requested target using dependency order.
- [x] 4.2 Given a check target is selected (`app|service|library|test`), when dependencies are declared, then Dusk Ocean shall build dependencies before running target tests.
- [x] 4.3 Given a repository has no `build` task, when `build` executes, then Dusk Ocean shall skip that target and print a skip message.
- [x] 4.4 Given a repository has no `test` task, when `check` executes, then Dusk Ocean shall skip that target and print a skip message.
- [x] 4.5 Given directory hash equals prior hash for a build/check hash file, when command executes, then Dusk Ocean shall skip execution as unchanged.
- [x] 4.6 Given check hash changed and build task exists, when build hash is stale/missing, then Dusk Ocean shall run build before check for the same target.
- [x] 4.7 Given a `check` command completes (pass or fail), when command output is captured, then Dusk Ocean shall write JUnit-formatted results as XML files directly inside `.ocean/results/` (not in any subdirectory), using the check label to derive the filename when no existing JUnit report is found.
- [x] 4.8 Given pass-through args are provided to `check`, when args are supplied after `--`, then Dusk Ocean shall append quoted pass-through args to the configured test command.

#### Constraints
- Pass-through args shall require `--` and shall reject positional args before separator.

### 5 Local Dependency Wiring
#### Context
`dusk-ocean add` and `dusk-ocean remove` wire and unwire local repository dependencies without prompts.

#### Requirements
- [x] 5.1 Given `dusk-ocean add --payload <lib> --target <repo>` is executed, when the payload and target are in the same app or share a common scope, then Dusk Ocean shall run the payload's `add` task in the target directory and register the dependency in workspace config.
- [x] 5.2 Given `add` would create a self-dependency, when validation runs, then Dusk Ocean shall reject the operation.
- [x] 5.3 Given the payload and target are in different apps and share no common scope, when validation runs, then Dusk Ocean shall reject with a scope-violation error.
- [x] 5.4 Given `dusk-ocean remove --payload <lib> --target <repo>` is executed, when the dependency exists, then Dusk Ocean shall run the payload's `uninstall` task in the target directory and remove the dependency entry from workspace config.
- [x] 5.5 Given the payload has no `uninstall` task in `ocean.config.json`, when `remove` executes, then Dusk Ocean shall fail with a missing-uninstall-command error.

#### Constraints
- Cross-app dependencies are permitted only when both payload and target share at least one scope name (see Section 7).

### 6 Package Installation
#### Context
`dusk-ocean install` runs the package manager install task for a specific local repository.

#### Requirements
- [x] 6.1 Given `dusk-ocean install --library <repo_name>` is executed, when the repo has an `install` task in `ocean.config.json`, then Dusk Ocean shall execute that install task from the repo's directory.
- [x] 6.2 Given the repo has no `install` task, when install runs, then Dusk Ocean shall skip and print a skip message.

### 7 Scope Management
#### Context
`add-scope` and `remove-scope` assign named group membership to repositories by writing to both `ocean.config.json` and workspace config. Scopes are the sole mechanism for permitting cross-app dependency relationships. A repo with no declared scope may only be depended on by repos within the same app, regardless of its directory location.

#### Requirements
- [x] 7.1 Given `dusk-ocean add-scope --scope-name <name> --target <repo>` is executed, when the target exists in workspace config, then Dusk Ocean shall add the scope name to the target's scope list in both `ocean.config.json` and workspace config.
- [x] 7.2 Given `dusk-ocean remove-scope --scope-name <name> --target <repo>` is executed, when the scope exists on the target, then Dusk Ocean shall remove the scope name from the target in both `ocean.config.json` and workspace config.
- [x] 7.3 Given a scope is removed from a target, when existing dependencies in workspace config relied on that scope for cross-app validation, then Dusk Ocean shall print a warning listing the affected dependency relationships.
- [x] 7.4 Given `dusk-ocean add --payload <lib> --target <repo>` is executed, when the payload has no declared scopes and the target is in a different app, then Dusk Ocean shall reject with a scope-violation error.

#### Constraints
- Two repos sharing at least one scope name are permitted to form a dependency regardless of app boundary or directory location.
- Directory location (`repos/libs/` vs `repos/apps/<app>/libs/`) does not grant or restrict dependency access.

### 8 Repository Rename
#### Context
`rename` renames a repository and propagates the change to all references throughout the workspace.

#### Requirements
- [x] 8.1 Given `dusk-ocean rename --repo <old-name> --new-name <new-name>` is executed without `--in`, when the target is an app, global library, or project and the new name is not already in use, then Dusk Ocean shall rename the repository directory, update `ocean.config.json`, update all workspace config references, update hash store paths, and update all dependency references that name the old repo.
- [x] 8.2 Given `dusk-ocean rename --repo <old-name> --new-name <new-name> --in <app-name>` is executed, when the target is a service or app library within the specified app and the new name is not already in use, then Dusk Ocean shall rename the repository directory, update `ocean.config.json`, update all workspace config references, update hash store paths, and update all dependency references that name the old repo.
- [x] 8.3 Given `rename` is run with a `--new-name` that conflicts with an existing repository name, when validation runs, then Dusk Ocean shall reject with a name-conflict error.
- [x] 8.4 Given `rename` is run with a `--repo` that does not exist in workspace config, when validation runs, then Dusk Ocean shall reject with a target-not-found error.
- [x] 8.5 Given `dusk-ocean menu` is executed, when the user selects `rename`, then Dusk Ocean shall prompt the user to select a repository type (app, global library, app library, app service, or project), prompt to select the specific repository within that type, prompt for a new name, and execute the rename operation.

### 9 Workspace Refresh
#### Context
`refresh` performs full dependency-ordered install/build/check across the workspace. Workspace registries are designed to be shared with collaborators whose GitHub access is the source of truth for which repos they may pull; refresh must therefore tolerate repos the local user cannot fetch and report what was and was not processed.

#### Requirements
- [x] 9.1 Given `dusk-ocean refresh` is executed, when workspace graph is non-empty, then Dusk Ocean shall run the `install` task (if present), then build, then check for each node in dependency order.
- [x] 9.2 Given a node has no install task, when refresh install phase runs, then Dusk Ocean shall skip install for that node and print a skip message.
- [x] 9.3 Given `--clear-hashes` is set, when refresh begins, then Dusk Ocean shall remove build/check hash records before refresh execution.
- [x] 9.4 Given refresh completes, when stale hashes remain for removed targets, then Dusk Ocean shall clean stale hash files.
- [ ] 9.5 Given a registered repo's local directory does not exist, when refresh's clone phase runs, then Dusk Ocean shall attempt the workspace `clone` task once and, if the clone fails for any reason, mark the repo `no-access`, continue with the remaining workspace nodes, and not return a hard error from that failure.
- [ ] 9.6 Given a node's transitive local dependency is unavailable (a dependency was marked `no-access`, was itself skipped for missing deps, or its repo directory is otherwise absent on disk), when refresh's install/build/check phases run, then Dusk Ocean shall skip those phases for the dependent node and record the unavailable dependency names in the run report.
- [ ] 9.7 Given refresh has completed all phases, then Dusk Ocean shall print a report grouping every workspace node into one of: `installed` (all phases ran or were no-op skipped), `no access` (clone unavailable), or `missing dependencies` (skipped due to absent local deps); the missing-dependencies group shall list, per node, which dependencies were unavailable.

#### Constraints
- Refresh shall fail on dependency graph cycles.
- A `no-access` or `missing-deps` skip shall not cause refresh to exit non-zero. Hard build/test failures on available nodes shall still surface as errors.

### 10 Container Publication
#### Context
`contain` builds and publishes service container images via flags, without interactive prompts. Rather than enforcing a specific containerization tool, Dusk Ocean executes the service's `contain` task (defined in `ocean.config.json`) after staging the build context. Before executing, Dusk Ocean substitutes reserved Dusk placeholders in the task command with runtime values. Before building, `contain` stages a minimal build context under `.ocean/stage/` by copying the service directory and its transitive local dependencies, mirroring their paths relative to the workspace root. Files matching patterns in `.oceanignore` are excluded. Files listed in `.oceaninclude` are copied to the staging root. Dusk Ocean computes a dependency-tree hash before contain; if the hash is unchanged since the last contain run, the build is skipped and the manifest is left marked clean.

Each service may declare two container-related fields in workspace config: `container_file` (path to the container build recipe, e.g. a Dockerfile or Containerfile) and `image_path` (full explicit registry path for the built image). These values are available as Dusk placeholders at contain-time.

#### Requirements
- [x] 10.1 Given `dusk-ocean contain --service <name>` is executed, when the service name matches exactly one service across all apps in workspace config, then Dusk Ocean shall resolve the service's `image_path` and `container_file`, stage the build context, substitute Dusk placeholders in the `contain` task command, and execute that command from the staging directory; if no `contain` task is defined, Dusk Ocean shall skip and print a skip message.
- [x] 10.2 Given `--service <name>` matches services in more than one app, when resolution runs, then Dusk Ocean shall fail with an ambiguity error instructing the user to add `--app <name>`.
- [x] 10.3 Given `dusk-ocean contain --app <name> --service <name>` is executed, when both flags are provided, then Dusk Ocean shall resolve the service's `image_path` and `container_file` from the specified app/service pair and proceed with placeholder substitution and task execution.
- [x] 10.4 Given the `contain` task exits with a non-zero code, when contain executes, then Dusk Ocean shall surface the error and shall not mark the manifest entry as clean.
- [x] 10.5 Given `contain` is executed, when staging the build context, then Dusk Ocean shall copy the service directory and each local dependency directory (resolved transitively from the workspace dep graph) into `.ocean/stage/`, preserving their paths relative to the workspace root, and shall exclude any files or directories matching patterns listed in `.oceanignore`; if `.oceanignore` is absent, the absence shall be logged and no patterns shall be applied.
- [x] 10.6 Given an `.oceaninclude` file exists at the workspace root, when staging the build context, then Dusk Ocean shall copy each file listed in `.oceaninclude` (relative paths from workspace root) to the staging root directory.
- [x] 10.7 Given `contain` executes, when the dependency-tree hash matches the previously recorded contain hash, then Dusk Ocean shall skip execution and set `contain_run` to `true` in the manifest without running the `contain` task.
- [x] 10.8 Given `contain` executes, when the dependency-tree hash differs from the previously recorded contain hash (or no prior hash exists), then Dusk Ocean shall run the `contain` task and, upon success, update the contain hash and set `contain_run` to `true` in the manifest.
- [x] 10.9 Given the `contain` task command contains Dusk placeholder tokens, when contain executes, then Dusk Ocean shall substitute the following tokens before invoking the command: `{{ocean:service_name}}` with the service name, `{{ocean:port}}` with the service's configured port, `{{ocean:image_path}}` with the service's `image_path` workspace config value, and `{{ocean:container_file}}` with the resolved absolute path to the service's `container_file`.

#### Constraints
- The `contain` task command shall have its stdout and stderr streamed directly to the CLI.
- The staging directory shall be removed after the `contain` task completes or fails.
- Services with no `contain` task are skipped; absence shall be logged.
- A bare filename for `container_file` (no directory component) shall resolve to `repos/containers/<name>`; a value containing path separators shall be treated as workspace-root-relative.
- `contain_run` in the manifest shall follow the same dirty/reset semantics as `build_run` and `check_run` (see Section 12).

### 11 Utility Commands
#### Context
The CLI provides a version visibility command.

#### Requirements
- [x] 11.1 Given `dusk-ocean version` is executed, when the command runs, then Dusk Ocean shall print the configured CLI version string.

### 12 Hash Command and Build Manifest
#### Context
`hash` registers repositories in `.ocean/manifest.json` without executing any build or test tasks. Each manifest entry records per-operation dependency-tree hashes (`build_hash`, `check_hash`, `contain_hash`) that are set when the corresponding operation completes successfully. To determine whether an operation is stale, the current dependency-tree hash is computed and compared against the stored value. The manifest gives scripts and agent workflows a fast, queryable signal for which repos need attention without re-running expensive operations.

#### Requirements
- [x] 12.1 Given `dusk-ocean hash` is executed without flags, when the command runs, then Dusk Ocean shall ensure a manifest entry exists for every registered repository in `.ocean/manifest.json`, without overwriting existing entries.
- [x] 12.2 Given `dusk-ocean hash --target <repo>` is executed, when the target exists in workspace config, then Dusk Ocean shall ensure a manifest entry exists for only that repository, leaving all other entries unchanged.
- [x] 12.3 Given a `build` command is about to execute for a repository, when the current dependency-tree hash matches the stored `build_hash` in the manifest, then Dusk Ocean shall skip the build.
- [x] 12.4 Given a `check` command is about to execute for a repository, when the current dependency-tree hash matches the stored `check_hash` in the manifest, then Dusk Ocean shall skip the check.
- [x] 12.5 Given a `build` command completes successfully for a repository, when a manifest entry exists for that repository, then Dusk Ocean shall store the dependency-tree hash as `build_hash` in the manifest entry.
- [x] 12.6 Given a `check` command completes successfully for a repository, when a manifest entry exists for that repository, then Dusk Ocean shall store the dependency-tree hash as `check_hash` in the manifest entry.
- [x] 12.7 Given `.ocean/manifest.json` does not exist, when `hash` executes, then Dusk Ocean shall create the file with initial entries for all registered repositories, with `build_hash`, `check_hash`, and `contain_hash` set to empty strings.
- [x] 12.8 Given a `contain` command completes successfully for a service, when a manifest entry exists for that service, then Dusk Ocean shall store the dependency-tree hash as `contain_hash` in the manifest entry.

#### Constraints
- The `hash` command shall not invoke any build or test task.
- Staleness is determined by comparing the current dependency-tree hash against the stored operation hash; a missing or empty hash is treated as stale.
- Manifest writes shall be atomic (write to a temp file, then rename) to prevent partial reads.
- The manifest format is JSON: a top-level object keyed by repository name, each value containing `kind` (string), `app` (string, optional), `name` (string), `build_hash` (string), `check_hash` (string), and `contain_hash` (string).

### 13 Library Move
#### Context
`dusk-ocean move` relocates a library repository from one location to another within the workspace. Supported moves are: app-scoped library to another app's library scope, app-scoped library to global library (`repos/libs/`), and global library to app-scoped library. The command updates the physical directory, workspace config, hash store paths, and all dependency references. Scope declarations are not altered automatically; the user is responsible for adjusting scopes after a move, though Dusk Ocean surfaces warnings when a move creates scope violations.

#### Requirements
- [x] 13.1 Given `dusk-ocean move --library <name> --from-app <app> --to-app <app>` is executed, when the library exists in the source app, then Dusk Ocean shall move the library directory from `repos/apps/<from-app>/libs/<name>/` to `repos/apps/<to-app>/libs/<name>/`, update workspace config to re-register it under the destination app, update all dependency references throughout workspace config, and update hash store paths.
- [x] 13.2 Given `dusk-ocean move --library <name> --from-app <app> --to-global` is executed, when the library exists in the source app, then Dusk Ocean shall move the directory to `repos/libs/<name>/`, re-register it as a global library in workspace config, update all dependency references, and update hash store paths.
- [x] 13.3 Given `dusk-ocean move --library <name> --from-global --to-app <app>` is executed, when the library exists as a global library, then Dusk Ocean shall move the directory to `repos/apps/<app>/libs/<name>/`, re-register it as an app-scoped library in workspace config, update all dependency references, and update hash store paths.
- [x] 13.4 Given a move would result in a name conflict at the destination, when validation runs, then Dusk Ocean shall reject with a name-conflict error.
- [x] 13.5 Given a library is moved from global scope to an app scope, when dependent repos in other apps relied on the library without a shared scope, then Dusk Ocean shall print a warning listing the affected dependency relationships that now violate scope constraints.

#### Constraints
- Move shall be atomic with respect to workspace config: directory rename and config update shall not leave the workspace in a partially-moved state on failure.
- Move shall not add or remove scopes automatically; scope adjustments are the user's responsibility, with violations surfaced per 13.5.

### 14 Application Run
#### Context
`dusk-ocean run` executes a user-defined `run` task for an application or service. Before executing the run task, Dusk Ocean performs hash-based pre-flight checks for build, check, and contain across all repos in the target's dependency tree. Any stale tasks are executed in dependency order (build → check → contain) before the run task begins. If any pre-flight task fails, the run task is not invoked.

#### Requirements
- [x] 14.1 Given `dusk-ocean run app --name <name>` is executed, when the app defines a `run` task, then Dusk Ocean shall perform pre-flight hash checks for build, check, and contain across all repos in the dependency tree and execute any stale tasks in dependency order before invoking the `run` task.
- [x] 14.2 Given a pre-flight build hash is stale for one or more repos, when `run` executes, then Dusk Ocean shall run `build` for the affected repo(s) in dependency order before proceeding.
- [x] 14.3 Given a pre-flight check hash is stale for one or more repos, when `run` executes, then Dusk Ocean shall run `check` for the affected repo(s) in dependency order before proceeding.
- [x] 14.4 Given a pre-flight contain hash is stale for one or more services and a `contain` task is defined, when `run` executes, then Dusk Ocean shall run `contain` for the affected service(s) before proceeding.
- [x] 14.5 Given any pre-flight task (build, check, or contain) exits with a non-zero code, when `run` is executing, then Dusk Ocean shall abort and not invoke the `run` task.
- [x] 14.6 Given a repo has no `run` task, when `dusk-ocean run` targets that repo, then Dusk Ocean shall skip and print a skip message.

#### Constraints
- Pre-flight checks shall respect the same dependency ordering used by build and check (Section 4).
- The run command shall be available both via flags and through the menu interface.

### 15 Projects
#### Context
Projects are standalone repositories that live under `repos/projects/` and are registered in `ocean.workspace.json`. Unlike apps and libraries, projects are not intended to be declared as dependencies of other repositories in the workspace. They represent self-contained tools, CLIs, research repos, or other non-library, non-application work. Projects participate in the standard build/check/install workflow and may themselves depend on global libraries.

#### Requirements
- [x] 15.1 Given `dusk-ocean add project` is executed, when a project name is provided and the name does not already exist under `repos/projects/`, then Dusk Ocean shall scaffold the project directory from the selected template and register it in workspace config under the `projects` list.
- [x] 15.2 Given `dusk-ocean menu remove` selects project type, when the user confirms deletion, then Dusk Ocean shall delete `repos/projects/<name>/` and unregister the project from workspace config.
- [x] 15.3 Given a project is registered in workspace config, when `build`, `check`, or `install` commands are executed, then Dusk Ocean shall include the project in dependency-ordered task execution according to its declared dependencies.
- [x] 15.4 Given a project is declared as a dependency target by any app, service, or library, when validation runs, then Dusk Ocean shall reject the operation with an error indicating projects cannot be used as dependencies.
- [x] 15.5 Given a project declares a dependency on a global library in workspace config, when the dependency is added, then Dusk Ocean shall permit the dependency if scope constraints are satisfied.

#### Constraints
- Projects shall not appear as a valid dependency source for apps, services, or libraries.
- Project names shall reject whitespace and follow the same character constraints applied to other repository types.
- Projects live exclusively under `repos/projects/` and shall not be co-located in `repos/apps/` or `repos/libs/`.

### 16 Variables
#### Context
Task commands in `ocean.workspace.json` and `ocean.config.json` may reference values through four namespaces: `{{env:NAME}}` (workspace-root `.env`), `{{var:NAME}}` (top-level `variables` map in `ocean.workspace.json`), `{{ocean:NAME}}` (system-reserved tokens), and `{{repo:NAME}}` (per-repo fields, re-evaluated against each target). Substitution is strict: missing keys and unknown namespaces are hard errors.

#### Requirements
- [x] 16.1 Given a task command containing `{{env:NAME}}`, `{{var:NAME}}`, `{{ocean:NAME}}`, or `{{repo:NAME}}` tokens, when Dusk Ocean executes the task, then Dusk Ocean shall replace each token with the value from the matching namespace.
- [x] 16.2 Given a task command contains a token whose key is missing from its namespace, when substitution runs, then Dusk Ocean shall fail with an error naming the namespace and key.
- [x] 16.3 Given a task command contains a token with an unknown namespace prefix, when substitution runs, then Dusk Ocean shall fail with an error naming the bad namespace.
- [x] 16.4 Given a workspace-root `.env` file exists, when `{{env:NAME}}` substitution runs, then Dusk Ocean shall load the file and resolve the token from the parsed `KEY=VALUE` pairs.
- [x] 16.5 Given the workspace-root `.env` file is absent, when `{{env:NAME}}` substitution runs, then Dusk Ocean shall log the absence and treat the env namespace as empty.
- [x] 16.6 Given a repo entry in `ocean.workspace.json` declares a `variables` map, when `BuildRepoVariables` runs, then Dusk Ocean shall merge those user variables onto the auto-derived reserved fields.
- [x] 16.7 Given a repo entry's `variables` block declares a key that collides with a reserved repo field name, when `ValidateWorkspaceConfig` runs, then Dusk Ocean shall reject the workspace config with an error naming the offending repo and key.

#### Constraints
- Reserved `{{repo:*}}` names are: `name`, `kind`, `path`, `scopes`, `remote` for every kind, plus `port`, `image_name`, `image_tag`, `dockerfile`, `container_file`, `image_path`, `app` for services, and `app` for app-scoped libraries.
- The four reserved `{{ocean:*}}` contain-time tokens (`service_name`, `port`, `image_path`, `container_file`) continue to be substituted by the contain command using literal `ReplaceAll` semantics so unknown tokens in a contain task are left verbatim rather than rejected.

### 17 Workspace Tasks
#### Context
`ocean.workspace.json` may declare a top-level `tasks` map: keys are task names, values are shell command templates that reference any combination of the four variable namespaces. Workspace tasks are invoked via `dusk-ocean task --name <task> --target <repo> [--app <app>]` and execute against a single repo. Iteration across multiple repos is not yet supported.

#### Requirements
- [x] 17.1 Given `ocean.workspace.json` declares a `tasks` map, when `dusk-ocean task --name <task> --target <repo>` is executed, then Dusk Ocean shall look up the named template, substitute its variable tokens against the target repo's context, and execute the resulting command from the workspace root.
- [x] 17.2 Given `--name` references a task that does not exist in `tasks`, when the command runs, then Dusk Ocean shall fail with a task-not-found error.
- [x] 17.3 Given `--target` references a repo that does not exist in workspace config, when the command runs, then Dusk Ocean shall fail with a target-not-found error suggesting `--app` for service or app-scoped library targets.
- [x] 17.4 Given `--target` matches multiple workspace entries (e.g. a project and a global library with the same name), when resolution runs without `--app`, then Dusk Ocean shall fail with an ambiguity error listing the matching kinds.
- [x] 17.5 Given a workspace task command exits with a non-zero status, when the task runs, then Dusk Ocean shall surface the exit error.

#### Constraints
- Workspace tasks execute one repo at a time. Iteration across all registered repos is intentionally deferred.
- The `Ocean` namespace is empty when running workspace tasks; only the four reserved contain-time tokens populate `{{ocean:*}}` and they only work inside contain tasks.

### 18 Polyrepo Remotes & Adopt/Register
#### Context
Each registered repo entry in `ocean.workspace.json` may carry a `remote` field — a plain string holding the upstream git URL — exposed to templates as `{{repo:remote}}`. The literal string `"None"` is a valid sentinel for entries whose upstream is unknown. Two new commands bring repos under management: `adopt` clones an external repo into the deterministic workspace path; `register` records an already-on-disk repo without cloning. Both commands write a starter `ocean.config.json` at the repo root and add a workspace entry. The presence of `ocean.config.json` at a repo root is Dusk Ocean's registration marker.

#### Requirements
- [x] 18.1 Given `dusk-ocean adopt <remote-url> --kind <kind> --name <name>` is executed and the deterministic target path does not exist, when adopt runs, then Dusk Ocean shall clone `<remote-url>` into that path, write a starter `ocean.config.json` at the repo root, and register the new entry in `ocean.workspace.json` with `remote` populated from `<remote-url>`.
- [x] 18.2 Given `adopt` runs and the target path already exists with no `ocean.config.json` inside, when the precondition check runs, then Dusk Ocean shall refuse with an error suggesting `register` instead.
- [x] 18.3 Given `adopt` runs and the target path already exists with an `ocean.config.json` inside, when the precondition check runs, then Dusk Ocean shall refuse with an already-registered error.
- [x] 18.4 Given `--name` is omitted from `adopt`, when the command runs, then Dusk Ocean shall derive `--name` from the basename of `<remote-url>` (stripping `.git`).
- [x] 18.5 Given `dusk-ocean register --kind <kind> --name <name>` is executed and the deterministic target path exists with no `ocean.config.json` inside, when register runs, then Dusk Ocean shall write a starter `ocean.config.json` and add the new entry in `ocean.workspace.json` with `remote` set from `--remote` or the literal string `"None"` when `--remote` is omitted.
- [x] 18.6 Given `register` runs and the deterministic target path does not exist, when the precondition check runs, then Dusk Ocean shall refuse with a not-found error.
- [x] 18.7 Given `register` runs and the deterministic target path exists with an `ocean.config.json` inside, when the precondition check runs, then Dusk Ocean shall refuse with an already-registered error.
- [x] 18.8 Given `--kind library` is supplied without `--app`, when adopt or register runs, then Dusk Ocean shall register the repo as a global library at `repos/libs/<name>/`.
- [x] 18.9 Given `--kind library` is supplied with `--app <app>`, when adopt or register runs, then Dusk Ocean shall register the repo as an app-scoped library at `repos/apps/<app>/libs/<name>/`.
- [x] 18.10 Given `--kind service` is supplied without `--app`, when flag validation runs, then Dusk Ocean shall reject the command.
- [x] 18.11 Given `--kind project` or `--kind app` is supplied with `--app`, when flag validation runs, then Dusk Ocean shall reject the command.

#### Constraints
- The folder layout under `repos/` is deterministic: there is no flag to relocate adopted or registered repos.
- Authentication for private clones is inherited from the user's ambient git credentials; Dusk Ocean stores no secrets.
- `register` does not read `.git/config` to auto-detect the remote URL; the user supplies it via `--remote` or accepts the `"None"` sentinel. This keeps the command tool-agnostic.

### 19 Templates as Workspace Entries
#### Context
Templates are scaffolding sources used by `dusk-ocean menu create` to stamp out new services, libraries, or projects. Each template is a directory under `repos/templates/<name>/` that may be registered in `ocean.workspace.json` as a top-level `WorkspaceTemplate` entry. A registered template carries a `kind` field declaring what it scaffolds and an optional `deps` list whose entries are automatically wired into anything scaffolded from the template via the same flow as `dusk-ocean add`. Templates are excluded from `build`, `check`, `install`, `run`, `contain`, `refresh`, and the hash manifest — they are not buildable artifacts. Apps are intentionally not template-able: the CLI scaffolds the app folder structure directly in code.

#### Requirements
- [x] 19.1 Given `dusk-ocean adopt <url> --kind template --name <n> --template-kind <k>` is executed, when validation passes, then Dusk Ocean shall clone the repo into `repos/templates/<n>/`, write a starter `ocean.config.json` whose `type` field is `<k>`, and add a `WorkspaceTemplate` entry to `ocean.workspace.json` with `kind=<k>` and the supplied remote URL.
- [x] 19.2 Given `dusk-ocean register --kind template --name <n> --template-kind <k>` is executed, when the deterministic path exists without an `ocean.config.json`, then Dusk Ocean shall write the starter config and add a `WorkspaceTemplate` entry just like `adopt` (without cloning).
- [x] 19.3 Given `--kind template` is supplied with `--template-kind app`, when validation runs, then Dusk Ocean shall reject the command with an error stating apps are not template-able.
- [x] 19.4 Given `--kind template` is supplied with no `--template-kind`, when validation runs, then Dusk Ocean shall reject the command with a missing-flag error.
- [x] 19.5 Given `--template-kind` is supplied with a `--kind` other than `template`, when validation runs, then Dusk Ocean shall reject the command.
- [x] 19.6 Given `dusk-ocean menu create` selects service or library type, when listing available templates, then Dusk Ocean shall include every workspace-registered template whose `kind` matches, plus any unregistered template directories under `repos/templates/` that classify themselves via `ocean.config.json`'s `type` field.
- [ ] 19.6.1 Given `dusk-ocean menu create` selects project type, when listing available templates, then Dusk Ocean shall include every workspace-registered template whose `kind` is `service`, `library`, or `project`, plus any unregistered template directories under `repos/templates/` whose `ocean.config.json` `type` field is one of those three kinds — so a project may scaffold from a library or service template in addition to a project template.
- [x] 19.7 Given a template registered in workspace config declares one or more entries in `deps`, when a repo is scaffolded from that template, then Dusk Ocean shall pre-validate every dep against the destination kind/app using the same scope, cycle, and flow rules as `dusk-ocean add` BEFORE any files are copied; if any dep would be illegal, the scaffold shall fail and no files shall be copied.
- [x] 19.8 Given the pre-validation check passes, when scaffold completes and the new repo is registered in workspace config, then Dusk Ocean shall propagate each declared dep by invoking the same wiring path as `dusk-ocean add`, running each dep's `add` task in the new repo's directory and recording the relationship in the workspace dependency graph.
- [x] 19.9 Given a template is registered in workspace config, when `build`, `check`, `install`, `run`, `contain`, `refresh`, or `hash` runs, then Dusk Ocean shall not include the template in the dependency graph or the hash manifest.
- [x] 19.10 Given a template is named as the `--payload` of `dusk-ocean add` (or as a dep source in any other repo's workspace entry), when validation runs, then Dusk Ocean shall reject the operation — templates cannot be used as dependencies.

#### Constraints
- Apps are not template-able. `--template-kind app` is rejected.
- Template kind values are restricted to `service`, `library`, and `project`.
- Templates participate in the workspace registry but are excluded from every build/check/refresh path.
- Filesystem-only templates (a directory under `repos/templates/<n>/` with no workspace entry) remain usable by `menu create` but cannot declare propagated `deps` — only registered templates participate in deps propagation.

