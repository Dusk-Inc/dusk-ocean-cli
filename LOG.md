# dusk-ocean-cli LOG

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
