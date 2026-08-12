# Provisioning

## Description

Provisioning is the lifecycle stage that puts every registered repo on disk at a known commit,
before any install, build, check, or contain runs. It is the stage that makes the workspace
reconstructible on a machine that has never seen it — a fresh developer laptop, and above all a CI
runner, which starts with nothing but this workspace's config repo.

Provisioning exists because Dusk is a polyrepo whose resolution context is not inside any Unit.
`pnpm-workspace.yaml`, `pnpm-lock.yaml`, `go.work`, and `ocean.workspace.json` live in the workspace
root repo, while the code lives in ~47 independently-remoted Unit repos that the root repo
gitignores. A Unit checked out alone has `"workspace:*"` dependencies that resolve to nothing. The
build cannot be moved to the Unit; the workspace must be assembled around it.

Half of this already exists. `refresh` walks the dependency graph and clones any missing node,
app shell, or non-code repo (`cloneNodeRepoIfMissing`, `cloneAppShellsIfMissing`,
`cloneRepoIfMissing`, `resolveNodeCloneTarget`), skipping a repo whose `remote` is empty or the
literal `"None"`. What it cannot do is target a **ref**: the workspace `clone` task is
`git clone {{repo:remote}} {{repo:path}}`, which always lands on the remote's default branch. A
feature spanning repos shares one branch name across every affected repo, and provisioning has no
way to ask for it.

This domain covers the `clone` command that closes that gap, the ref resolution it performs, and
the lock file it emits.

## Blocking defect

`ocean.workspace.json` declares two tasks that reference a variable with no source:

```json
"checkout_existing": "git -C {{repo:path}} checkout {{repo:branch}}",
"checkout_new":      "git -C {{repo:path}} checkout -b {{repo:branch}}"
```

`BuildRepoVariables` populates the reserved repo namespace with `name`, `kind`, `path`, `scopes`,
`remote`, and (for app-scoped kinds) `app`. `branch` is not among them, no repo declares it under
its `variables` map, and `Substitute` returns a hard error — `missing variable repo:branch` — on any
token it cannot resolve. Both tasks therefore fail whenever they are reached through
`RunWorkspaceTaskAt`.

Provisioning cannot be built on top of a broken checkout task, so the fix is the first unit of work
in this domain: `branch` becomes a reserved repo variable fed by the caller at task-invocation time,
defaulting to empty only where the task template does not reference it.

## Policies

### Enumeration

- **Registered-only.** `clone --all` clones exactly the repos registered in `ocean.workspace.json` —
  apps, their services / libraries / projects / testing repos, global libraries, projects,
  templates, infrastructure, and docs. A directory under `repos/` that is not registered is never
  touched. `repos/sandbox/` is not a registered kind and is never provisioned.
- **Remote-required.** A repo whose `remote` is empty or the literal `"None"` is skipped with a
  note, never an error. Six entries are in that state today; a workspace must stay provisionable
  while some of its Units have no upstream yet.
- **Shared-path.** Enumeration and destination-path resolution reuse `refresh`'s existing helpers
  rather than reimplementing them. Provisioning changes *which ref* a repo lands on, not *where* it
  lands; two implementations of the path rule would drift.

### Ref resolution

- **Ordered-candidates.** For each repo, the ref is the first candidate that exists as a remote
  head: the `--ref` value, then each `--fallback` in the order given, then the remote's default
  branch. Existence is tested with `git ls-remote --heads <remote> <candidate>`, so a candidate that
  does not exist costs one network round trip and no clone.
- **Per-repo.** Resolution runs independently per repo. `--ref feat-42 --fallback dev` lands the
  three repos a Feature touches on `feat-42` and every other repo on `dev` — which is what makes a
  cross-repo Feature build as one unit under the shared branch name.
- **Default-branch-terminal.** The remote's default branch is always the last candidate and is never
  skipped, so resolution cannot fail for a repo that exists.
- **Pinned-exact.** `--pin <name>=<sha>` overrides resolution for one repo and checks out that
  commit detached. It takes precedence over `--ref` and every fallback. A `--pin` naming an
  unregistered repo is an error; a `--pin` whose sha is not reachable on the remote is an error.
- **Locked-exact.** `--from-lock <path>` overrides resolution for *every* repo, checking each out at
  the sha the lock records. `--ref`, `--fallback`, and `--pin` are usage errors alongside it — a
  lock is a complete answer, and silently letting a flag amend it would make the lock a lie.

### Materialization

- **Idempotent.** A repo already on disk is fetched and checked out to the resolved ref rather than
  re-cloned. Running `clone --all` twice produces the same tree state, so the command is usable on a
  warm CI runner and on a developer machine mid-work alike.
- **Non-destructive.** A repo whose working tree has uncommitted changes is left untouched and
  reported, never reset or stashed. Provisioning does not get to discard a developer's work; a
  human resolves it.
- **Concurrent.** `--jobs N` clones up to N repos in parallel, defaulting to 4. Clones are
  independent — no repo's clone depends on another's — so the stage is network-bound and serializing
  it dominates cold-runner wall clock.
- **Fail-closed.** Any repo that fails to clone or check out fails the command with a non-zero exit
  after all other repos are attempted, so one bad remote surfaces the whole set of failures rather
  than the first one.

### Lock

- **Emitted.** `--lock <path>` writes the resolved state of the run: for each repo, its `kind`,
  `app`, `remote`, resolved `ref`, and the exact `sha` checked out, plus the workspace name and the
  `--ref` / `--fallback` inputs that produced it.
- **Complete.** The lock records every repo that was materialized, including those that resolved to
  a fallback. A repo skipped for having no remote is recorded with a null sha and a `skipped` reason,
  so the lock always accounts for the full registered set.
- **Round-trips.** `clone --all --from-lock <path>` reproduces the exact tree the lock describes.
  This is the mechanism by which a promotion into staging builds the same commits CI validated,
  rather than whatever the branches point at by then.
- **Uncommitted by default.** The lock is a run artifact, not workspace config. It is not written
  unless `--lock` is passed, and `ocean.workspace.lock.json` belongs in the workspace `.gitignore`
  until a promotion flow has reason to commit one deliberately.

## Command surface

```bash
dusk-ocean clone --all [--ref <branch>] [--fallback <branch>]... [--pin <name>=<sha>]...
                       [--lock <path>] [--jobs <n>]
dusk-ocean clone --all --from-lock <path> [--jobs <n>]
dusk-ocean clone --target <name> [--app <app>] [--ref <branch>]
```

`--target` is the single-repo form, and is what `refresh`'s clone-if-missing path becomes once ref
resolution exists. `--all` without `--ref` behaves as today's `refresh` clone pass does: default
branch for everything.

## Lock format

```json
{
  "workspace": "dusk-workspace",
  "resolved_with": { "ref": "feat-42", "fallback": ["dev"] },
  "repos": [
    {
      "name": "dusk-iris",
      "kind": "library",
      "app": "",
      "path": "repos/libs/dusk-iris",
      "remote": "https://github.com/Dusk-Inc/dusk-iris",
      "ref": "feat-42",
      "sha": "9f2c1ab4e7d3f5602b8c1d94a7e3f0b52c6d8a14"
    },
    {
      "name": "plexus-keys",
      "kind": "app",
      "app": "",
      "path": "repos/apps/plexus-keys",
      "remote": "https://github.com/Dusk-Inc/plexus-keys",
      "ref": "dev",
      "sha": "3ba7d10c58e94f26a1d7b0c3e85f492a6d1c7f38"
    }
  ]
}
```

No timestamp field: a lock is identified by its content, and a timestamp would make two locks over
identical commits compare unequal and defeat cache keying on its hash.

## Relationship to caching

`.ocean/manifest.json` stores per-operation dependency-tree hashes and is gitignored, so a cold CI
runner has no hashes and rebuilds the entire graph. Restoring `.ocean/` on a runner turns the
existing hash logic into affected-set computation for free — only repos whose dependency tree
changed re-run. The lock's content hash is the correct cache key, because it changes exactly when
the resolved source changes. This requires no new hashing work in the CLI; it is a property of the
lock existing.

## Testing

| Set | Cases |
| :--- | :--- |
| **Domain** | Every registered repo clones at `--ref`; a repo lacking that branch falls back; a lock round-trips to an identical tree; a second run is a no-op. |
| **Boundary** | Empty workspace; a workspace where no repo has the `--ref` branch; `--jobs 1`; `--jobs` above the repo count; a repo already on disk at the correct sha. |
| **Error** | Unresolvable remote; `--pin` naming an unregistered repo; `--pin` sha not on the remote; `--from-lock` with `--ref`; malformed lock; dirty working tree. |
| **Chaos** | Lock with a null sha; remote returning a partial `ls-remote`; a branch name containing shell metacharacters, which must not reach a shell unescaped through the workspace task template. |

The chaos row is load-bearing: workspace tasks execute through `bash -lc`, so a ref taken from a CI
payload is attacker-adjacent input reaching a shell. Ref values must be validated against
`git check-ref-format` before substitution.
