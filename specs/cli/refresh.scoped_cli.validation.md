# Validation Evidence — refresh.scoped_cli

Evidence for the `realization → validation` advance of the [refresh.scoped_cli](refresh.scoped_cli.md)
boundary: the realized command verified against its contract under real conditions.

- **Date:** 2026-06-01
- **Artifact:** the real `dusk-ocean` binary, built from this repo via `go build .` (the project's
  build task), invoked as a process — not an in-process shim.
- **Environment (contained-but-real):** a scratch workspace with a real `ocean.workspace.json`
  and a deliberate dependency graph — global libs `core` (no deps) ← `util`; projects `app-cli`
  (→ `util`) and `standalone` (unrelated); non-code `infra-repo` / `docs-repo`. Repo directories
  were pre-created with empty-task `ocean.config.json`, so clone is skipped (dirs present) and
  install/build/check emit their real "skipped" lines — exercising the real config-read, graph
  walk, and process edges (argv → stdout/stderr → exit code) without heavyweight side effects.
  A second scratch workspace held a `a ↔ b` cycle to exercise the cycle failure mode.

## Observed process edges

### Selection / success paths (exit 0)

- `refresh --repo app-cli` → emits, in dependency order, `core` then `util` then `app-cli`
  (install/build/check skipped lines); **no `standalone`**, **no `infra-repo`/`docs-repo` clone**
  attempt — non-code repos excluded from a scoped run. Exit `0`.
- `refresh --repo app-cli --no-deps` → only `app-cli`; dependencies omitted. Exit `0`.
- `refresh --repo util --clear-hashes` → `core` then `util`; `--clear-hashes` composes (no
  `.ocean/hashes` present in the scratch workspace, so clearing is a no-op and prints nothing,
  which is the correct behavior when there is nothing to clear). Exit `0`.
- `refresh --repo standalone` → only `standalone` (unrelated repo with no deps). Exit `0`.
- `refresh --repo board_education` (live workspace; `board_education` is an app registered with
  a remote but with no services/libraries/tests) → `cloning board_education` + the clone task
  runs and clones it into `repos/apps/board_education`. Exit `0`. Re-running with the repo now
  present is a clean no-op (exit `0`, no output). This is the v0.2.0 behavior: a registered repo
  with a remote is cloned even with no declared components (clone-only). (Earlier iterations
  during validation first misreported this app as "not found", then skipped it as "no buildable
  components"; the contract was amended to clone-by-registration after the requester confirmed
  the app must be cloned regardless of declared components.)

### Failure modes — each exits non-zero with a stderr diagnostic

| Failure mode (contract) | Invocation | Exit | stderr |
| --- | --- | --- | --- |
| `--no-deps` without `--repo` (usage error) | `refresh --no-deps` | `1` | `Error: --no-deps requires --repo: no target to strip dependencies from` |
| unknown `--repo` (names the repo) | `refresh --repo ghost` | `1` | `Error: repo "ghost" not found in workspace config` |
| dependency cycle among targeted repos | `refresh --repo a` (a↔b) | `1` | `Error: dependency cycle detected` |

The non-zero exits depend on the CLI-wide exit-code fix made during realization (`main` now
`os.Exit(1)`s when `Execute` returns an error); before it, every failure exited `0`.

## Contract-test corroboration

The process-edge runs above are backed by the contract tests committed in realization
(`src/functions/scoped_refresh_test.go`, `src/commands/refresh_test.go`): scope ordering,
transitive-dep inclusion, non-code exclusion, `--no-deps`, the usage error, the unknown-repo
error (asserting the message names the repo), and the cycle error — across the
domain/boundary/error/chaos sets. `go test ./...` is green.

## Verdict

The realized command satisfies `contract.inputs` / `contract.outputs` and maps all three
declared `failure_modes` to their non-zero exits with stderr diagnostics, verified against the
real binary. `verified_under_real_conditions` is met.
