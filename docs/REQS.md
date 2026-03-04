# Dusk Ocean Requirements
Dusk Ocean is a polyglot monorepo CLI for scaffolding repositories, managing dependency wiring, and executing cached build/check workflows. Most commands are fully flag-driven and composable in scripts or agent workflows.

## Requirements
### 1 Workspace Initialization
#### Context
`dusk-ocean init` bootstraps the workspace root and baseline repository layout.

#### Requirements
- [x] 1.1 Given `dusk-ocean init` is run with `--name`, when `ocean.workspace.json` does not exist, then Dusk Ocean shall create it with the provided workspace name, default allowed ports (`3000-3999`), and empty apps and libraries lists.
- [ ] 1.2 Given `dusk-ocean init` is run, when baseline directories are missing, then Dusk Ocean shall create `.ocean`, `.ocean/results`, `.ocean/hashes`, `repos`, `repos/apps`, `repos/libs`, and template directories under `repos/templates/apps` (including `services/`, `jobs/`, `libs/`, and `docs/` subfolders).
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
- [x] 3.2 Given `dusk-ocean menu create` selects app type, when prompts complete, then Dusk Ocean shall scaffold `repos/apps/<name>/` with `services/`, `jobs/`, `libs/`, and `docs/` subdirectories and register it in workspace config.
- [x] 3.3 Given `dusk-ocean menu create` selects library type, when the user selects workspace-level or app-adjacent placement and provides a name, then Dusk Ocean shall scaffold `repos/libs/<name>/` or `repos/apps/<app>/libs/<name>/` accordingly and register it in workspace config.
- [x] 3.4 Given template files include `{{placeholder}}` tokens, when scaffolding occurs, then Dusk Ocean shall prompt for replacement values and apply them to both file names and file contents.
- [x] 3.5 Given `dusk-ocean menu remove` is executed, when the user selects a target and confirms deletion, then Dusk Ocean shall delete the target directory and unregister it from workspace config.
- [x] 3.6 Given `menu remove` targets a library that has dependents, when deletion is confirmed, then Dusk Ocean shall run uninstall tasks for all dependent repos before removing the library and pruning dependency references from workspace config.
- [x] 3.7 Given user confirmation is not `y` in any `menu remove` flow, when the prompt resolves, then Dusk Ocean shall abort without mutation.

#### Constraints
- Scaffolding (`menu create`) and repo deletion (`menu remove`) are only available through the menu and have no flag-based equivalents.
- All other commands available through the menu must also be fully executable via flags.
- Repository type names shall reject whitespace and only allow configured character sets.

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
- [x] 8.1 Given `dusk-ocean rename --repo <old-name> --new-name <new-name>` is executed, when the target exists and the new name is not already in use, then Dusk Ocean shall rename the repository directory, update `ocean.config.json`, update all workspace config references, update hash store paths, and update all dependency references that name the old repo.
- [x] 8.2 Given `rename` is run with a `--new-name` that conflicts with an existing repository name, when validation runs, then Dusk Ocean shall reject with a name-conflict error.
- [x] 8.3 Given `rename` is run with a `--repo` that does not exist in workspace config, when validation runs, then Dusk Ocean shall reject with a target-not-found error.

### 9 Workspace Refresh
#### Context
`refresh` performs full dependency-ordered install/build/check across the workspace.

#### Requirements
- [x] 9.1 Given `dusk-ocean refresh` is executed, when workspace graph is non-empty, then Dusk Ocean shall run the `install` task (if present), then build, then check for each node in dependency order.
- [x] 9.2 Given a node has no install task, when refresh install phase runs, then Dusk Ocean shall skip install for that node and print a skip message.
- [x] 9.3 Given `--clear-hashes` is set, when refresh begins, then Dusk Ocean shall remove build/check hash records before refresh execution.
- [x] 9.4 Given refresh completes, when stale hashes remain for removed targets, then Dusk Ocean shall clean stale hash files.

#### Constraints
- Refresh shall fail on dependency graph cycles.

### 10 Container Publication
#### Context
`contain` builds and publishes service container images via flags, without interactive prompts. Before building, `contain` stages a minimal build context under `.ocean/stage/` by copying the service directory and its transitive local dependencies, mirroring their paths relative to the workspace root. Files matching patterns in `.oceanignore` are excluded. Files listed in `.oceaninclude` are copied to the staging root.

#### Requirements
- [x] 10.1 Given `dusk-ocean contain --service <name>` is executed, when the service name matches exactly one service across all apps in workspace config, then Dusk Ocean shall resolve the image reference, stage the build context, run `docker build -t <image> -f <dockerfile> .` from the staging directory, then run `docker push <image>`.
- [x] 10.2 Given `--service <name>` matches services in more than one app, when resolution runs, then Dusk Ocean shall fail with an ambiguity error instructing the user to add `--app <name>`.
- [x] 10.3 Given `dusk-ocean contain --app <name> --service <name>` is executed, when both flags are provided, then Dusk Ocean shall resolve the image reference from the specified app/service pair and proceed with build and push.
- [x] 10.4 Given the local image build fails, when contain executes, then Dusk Ocean shall surface the build error and not attempt push.
- [x] 10.5 Given `contain` is executed, when staging the build context, then Dusk Ocean shall copy the service directory and each local dependency directory (resolved transitively from the workspace dep graph) into `.ocean/stage/`, preserving their paths relative to the workspace root, and shall exclude any files or directories matching patterns listed in `.oceanignore`; if `.oceanignore` is absent, the absence shall be logged and no patterns shall be applied.
- [x] 10.6 Given an `.oceaninclude` file exists at the workspace root, when staging the build context, then Dusk Ocean shall copy each file listed in `.oceaninclude` (relative paths from workspace root) to the staging root directory.

#### Constraints
- Container build/push shall stream command output directly to CLI stdout/stderr.
- The staging directory is removed after `docker push` completes or after a build failure.

### 12 Hash Command and Build Manifest
#### Context
`hash` computes directory hashes for all registered repositories (or a specified target) without executing any build or test tasks. Results are written to `.ocean/manifest.json`, which records each repository's current hash, whether that hash has changed since it was last recorded (dirty), and whether a build or check has been run since the current hash was established. The manifest gives scripts and agent workflows a fast, queryable signal for which repos need attention without re-running expensive operations.

#### Requirements
- [x] 12.1 Given `dusk-ocean hash` is executed without flags, when the command runs, then Dusk Ocean shall compute the directory hash for every registered repository and write the results to `.ocean/manifest.json`.
- [x] 12.2 Given `dusk-ocean hash --target <repo>` is executed, when the target exists in workspace config, then Dusk Ocean shall compute and update the hash entry for only that repository in the manifest, leaving all other entries unchanged.
- [x] 12.3 Given a hash is computed for a repository, when the computed hash differs from the previously recorded hash in the manifest, then Dusk Ocean shall mark the repository as dirty and reset `build_run` and `check_run` to `false` in the manifest entry.
- [x] 12.4 Given a hash is computed for a repository, when the computed hash matches the previously recorded hash, then Dusk Ocean shall preserve the existing `build_run` and `check_run` values in the manifest.
- [x] 12.5 Given a `build` command completes successfully for a repository, when a manifest entry exists for that repository, then Dusk Ocean shall set `build_run` to `true` in the manifest entry.
- [x] 12.6 Given a `check` command completes successfully for a repository, when a manifest entry exists for that repository, then Dusk Ocean shall set `check_run` to `true` in the manifest entry.
- [x] 12.7 Given `.ocean/manifest.json` does not exist, when `hash` executes, then Dusk Ocean shall create the file with initial entries for all computed repositories, with `dirty` set to `true`, `build_run` set to `false`, and `check_run` set to `false`.

#### Constraints
- The `hash` command shall not invoke any build or test task.
- `dirty` shall be `true` if and only if the computed hash differs from the previously stored hash; a missing prior entry shall be treated as a hash mismatch.
- Manifest writes shall be atomic (write to a temp file, then rename) to prevent partial reads.
- The manifest format is JSON: a top-level object keyed by repository name, each value containing `hash` (string), `dirty` (bool), `build_run` (bool), and `check_run` (bool).

### 11 Utility Commands
#### Context
The CLI provides a version visibility command.

#### Requirements
- [x] 11.1 Given `dusk-ocean version` is executed, when the command runs, then Dusk Ocean shall print the configured CLI version string.
