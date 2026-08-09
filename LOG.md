# dusk-ocean-cli LOG

## Made the `test` unit kind registerable
Every consumer of an app's `testing/` unit was already built — `WorkspaceTest` with its own
`deps[]`, `MakeTestNode`, `FindAppTestIndex`, `AddTestToWorkspace`, scope add/remove, rename,
uninstall, and `RegisterDiscoveredAppSubRepos`' `AppSubRepoKindTest` branch — but nothing could
put an entry in the workspace to begin with, so `check test` / `build test` answered
`test not registered in workspace` for a unit that was sitting on disk. Two separate dead ends:

1. `register` had no `test` kind, and the only code path that registers a test — app
   sub-repo discovery — runs solely on `add`/`adopt`/`register` of an **app**, all of which
   refuse an app that is already registered. A test unit added to an existing app was therefore
   unreachable, and the one in the workspace had been recorded by hand.
2. `add test` calls `ListTemplatesByType("test")`, but `ValidateTemplateKind` rejected `test`, so
   no test template could ever be registered. The command half-worked through
   `ListTemplatesByType`'s on-disk fallback, with no dep propagation from the template.

- **src/tokens/workspace.go** — added `RepoKindTest` and `TemplateKindTest`.
- **src/functions/repo_kind.go** — `ResolveRepoPath` resolves `test` to
  `repos/apps/<app>/testing/<name>` and requires `--app`; `ValidateRepoKindFlags` groups it with
  `service` as app-required; `ValidateTemplateKind` accepts `test`. Both error messages widened.
- **src/functions/register.go** — `IsRegisteredInWorkspace`, `registerEntryInWorkspace` (via
  `AddTestToWorkspace`), and `setRemoteOnRepo` each gained a `test` branch. `adopt` shares all
  three, so it gained the kind at the same time.
- **src/functions/starter_config.go** — `starterConfigTypeFromKind` maps `test` to `type: "test"`,
  so a unit registered without an `ocean.config.json` gets a correct starter.
- **src/commands/register.go** — `--kind`, `--app`, and `--template-kind` help text, and the
  `Long` description's list of allowed locations.
- **src/commands/menu.go** — `promptForRepoKind` offers `test` and prompts for its parent app.
  Its template-kind list was also missing `app`, which `ValidateTemplateKind` has always
  accepted and `base-app` is registered under; added both.
- **src/functions/{repo_kind,register,templates}_test.go** — path resolution and `--app` cases for
  the new kind; `RegisterRepo` cases for the happy path (asserting the starter type, the remote,
  and that the entry resolves through `MakeTestNode`), duplicate rejection, and missing `--app`;
  `test` added to the accepted-template-kind set.

Verified: `go build ./...`, `go vet ./...`, `go test ./...` clean. End to end,
`register --kind test --name auth-e2e-ts --app plexus-auth` followed by two
`add --payload <lib> --target auth-e2e-ts` calls produced a workspace entry matching the
hand-written one it replaces, and `check test --name auth-e2e-ts --in plexus-auth` ran the suite
green three times over.

## Extended `--group` overrides to `run`/`stop` for services and apps
Feature #65 shipped deployment-mode task overrides but only the project lifecycle path
(`run`/`stop`/`check project`) honored `--group`; `run`/`stop` for services and apps read the
base task directly and ignored the flag. This completes the gap so a service can declare a
`test` override group whose `run`/`stop` stand an ephemeral dependency stack up/down.

- **src/functions/run.go** — added `repoVariableContext(...)` (generalizes
  `projectVariableContext` to any repo kind) and `resolveRepoTask(...)` (reads the repo config,
  `ValidateOverrides`, builds the variable context, and calls `ResolveGroupCommand`).
  `RunService`/`RunApp` now take a `models.GroupSelection` and resolve `run` through it. A
  non-base `--group` run **skips the build/check/contain pre-flight** — the group command is the
  whole lifecycle for that mode (a `test` group must not `pnpm test`/`docker build && push` the
  service before standing up its deps); base `run` preflights exactly as before.
- **src/functions/stop.go** — `StopService`/`StopApp` take a `models.GroupSelection` and resolve
  `stop` through `resolveRepoTask`.
- **src/commands/{run,stop}.go**, **src/commands/menu.go** — pass `groupSelection(cmd)` into the
  four functions (menu paths pass base).
- **src/functions/run_test.go** — `TestServiceGroupResolution`: base uses the plain task, a
  `test` group overrides `run` and `stop`, and an unknown group errors.

Verified: `go build ./...`, `go test ./...`, `go vet ./...`, and `gofmt -l` on the touched files
all clean.

## Completed the git-init + optional-remote VCS workflow
The VCS-bootstrap workflow was half-finished: `WireNewRepoVcsAt` (src/functions/vcs.go) ran
`create_remote` then `checkout_new` and never ran the `init` task, so it operated on a non-git
directory in the wrong order, and `checkout_new` referenced the unresolvable `{{repo:branch}}`
variable. `register` never wired VCS at all.

- **src/functions/vcs.go** — `WireNewRepoVcsAt` now delegates to a shared `wireRepoVcsAt(...,
  createRemote bool)` running the correct local-first order: `init` → `initial_commit` →
  `create_remote`. `checkout_new` is dropped (its branch-birth job is done by `git init -b main`
  in the `init` task). A failed `create_remote` is non-fatal (warns on stderr) so the local repo
  still initializes when `gh`/`org` are unavailable. Added `InitRepoVcs`/`InitRepoVcsAt` for an
  already-registered repo: validates registration + on-disk dir, guards against an existing
  `.git`, runs the sequence, and records the derived `<org>/<name>` (or `--remote` override, or
  `None` for `--no-remote`) into the per-repo `remote` field via `setRemoteOnRepo`.
- **src/tokens/workspace.go** — added `WorkspaceTaskInitialCommit = "initial_commit"` and
  appended it to `DefaultWorkspaceTaskNames` so `init` seeds an empty slot for it.
- **src/commands/init_repo.go** — new `init-repo` command (`--kind/--name/--app/--remote/
  --no-remote`); registered in src/commands/root.go.
- **src/functions/vcs_test.go** — rewrote the two order-asserting cases to the new sequence and
  the non-fatal-remote semantics; added `TestInitRepoVcsAt` (no-remote, derived org/name,
  unregistered, existing-.git, org-absent warning).

The canonical task commands live in the workspace's `ocean.workspace.json`:
`init`=`git init -b main {{repo:path}}`, `initial_commit`=`git -C {{repo:path}} add -A && git -C
{{repo:path}} commit -m "chore: initial commit"`, `create_remote` unchanged.

Verified: `go build ./...`, `go test ./...`, `gofmt`, `go vet` all clean; e2e
`init-repo --kind library --name dusk-ui-audit --no-remote` created `.git` on `main` with an
initial commit and recorded `remote: None`, and re-running errors as already-a-git-repo.

## [test] feat-65 core-cli integration gate (build-feat-65, HEAD c7e82e5)
- #145 valid overrides config parses and registers group — integration test asserts Then end-to-end vs real dusk-ocean binary + real ocean.config.json + real .ocean/manifest.json; PASS stable x3.
- #146 duplicate group name is a validation error — Then asserted end-to-end vs real deps; PASS stable x3.
- #147 group entry with no name is a validation error — Then asserted end-to-end vs real deps; PASS stable x3.
- #148 group override of an unknown base task is a validation error — Then asserted end-to-end vs real deps; PASS stable x3.
- #149 --group selects a group's task command — Then asserted end-to-end vs real deps; PASS stable x3.
- #150 unlisted task under a group inherits the base command — Then asserted end-to-end vs real deps; PASS stable x3.
- #151 no group selected runs the base command — Then asserted end-to-end vs real deps; PASS stable x3.
- #152 unknown --group value is a hard error — Then asserted end-to-end vs real deps; PASS stable x3.
- #153 overrides apply to non-build lifecycle tasks — Then asserted end-to-end vs real deps; PASS stable x3.
- #154 group commands resolve tokens like base tasks — Then asserted end-to-end vs real deps; PASS stable x3.
- #155 operation under a group writes the per-(repo,group) hash slot — Then asserted end-to-end vs real deps; PASS stable x3.
- #156 base mode uses a slot distinct from any group's — Then asserted end-to-end vs real deps; PASS stable x3.
- #157 changing one group's override invalidates only that group's cache — Then asserted end-to-end vs real deps; PASS stable x3.
- #158 a matching group-slot hash skips the operation as fresh — Then asserted end-to-end vs real deps; PASS stable x3.
- #159 a missing or mismatched group-slot hash rebuilds — Then asserted end-to-end vs real deps; PASS stable x3.
- #160 run honors --group but writes no cache slot — Then asserted end-to-end vs real deps; PASS stable x3.
- #161 stop honors --group but writes no cache slot — Then asserted end-to-end vs real deps; PASS stable x3.
