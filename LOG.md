# dusk-ocean-cli LOG

## Documented the refresh, variables, and workspace-task functions
`comment-guard` refused a commit touching these three files: 14 functions carried no godoc and
two decorative comment blocks sat inside `runRefreshNodes`. Both rules are CLAUDE.md's, hard-gated
at commit time, and the debt predated the gate rather than arriving with the refresh work.

- **src/functions/refresh.go** — godocs on `RunRefresh`, `refreshNodeLabel`, `RunInstall`,
  `resolveNodeCloneTarget`, `cloneNodeRepoIfMissing`, and `runInstall`. The two in-body comments
  describing the clone and missing-deps passes moved into `runRefreshNodes`'s own godoc, which is
  where that context belongs — it explains the function's two-pass shape and why the topological
  order is what makes the second pass sound.
- **src/functions/variables.go** — godocs on `Substitute`, `LoadEnvFile`, `LoadWorkspaceVariables`,
  `BuildRepoVariables`, and `mergeRepoVariables`, each recording the error-over-silence choice it
  makes: an unresolved token fails rather than expanding empty, a missing `.env` is tolerated, and
  a repo variable colliding with a reserved field is refused rather than overwriting it.
- **src/functions/workspace_tasks.go** — godocs on `RunWorkspaceTask`, `RunWorkspaceTaskAt`, and
  `ResolveRepoKindByName`, including why an ambiguous target name is an error rather than a
  first-match win.

`comment-guard` itself was fixed in the same pass (workspace repo, not this one): it modelled a
godoc as a single `//` line directly above a `func`, so multi-line godocs and doc-comments on
`const`/`var`/`type` members were refused. 31 of the 47 findings here were that defect.

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
