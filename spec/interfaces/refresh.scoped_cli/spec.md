---
id: refresh.scoped_cli
type: cli
name: Scoped Refresh Command
status: check
owner: dusk-ocean
version: 0.2.0
contract:
  inputs:
    command: refresh
    stdin: none
    flags:
      "--repo": "string (workspace repo name); scope the refresh to one repo. Omitted -> whole-workspace refresh (current behavior, unchanged)."
      "--no-deps": "boolean; with --repo, refresh only the named repo and skip its transitive dependencies."
      "--clear-hashes": "boolean (existing); clear all build/check hashes before refreshing."
  outputs:
    stdout: "human-readable progress, one line per repo in dependency order (e.g. 'cloning <name>', 'install skipped for <label>: no install command')."
    stderr: "diagnostics, plus the forwarded output of any failed install/build/check subprocess."
    exit_codes:
      "0": "every targeted repo cloned (if missing), installed, built, and checked successfully."
      "non-zero": "usage error, unknown repo, dependency cycle, or any clone/install/build/check step failed."
  failure_modes:
    - "--repo names a repo absent from the workspace config -> non-zero exit; message names the repo."
    - "--no-deps supplied without --repo -> non-zero usage error."
    - "dependency cycle among the targeted repos -> non-zero exit."
---

## Summary

`refresh` brings workspace repositories to a ready state: it clones any that are missing
(where a remote is configured and permitted), then runs each repo's install, build, and check
tasks in dependency order. Today it always operates over the **entire** workspace graph.

This boundary adds a **scoped** invocation: `--repo <name>` narrows the run to a single
repository and, by default, its transitive dependency tree — the minimal set that repo needs
to build and check. It exists because refreshing the whole workspace is slow and noisy when a
developer is working in one corner of the monorepo and only needs that repo (and the libraries
it consumes) up to date. The command is run by a human at a terminal and by CI steps that
prepare a single target before building it.

## Contract

### Invocation

`refresh` takes no positional arguments; the target is named with the `--repo` flag so the
invocation reads explicitly and stays consistent with the existing `--clear-hashes` flag.
All flags are optional, and the default (no flags) is unchanged: a full whole-workspace
refresh. The command reads no stdin.

- **`--repo <name>`** selects one repository by its **workspace name** (the `Name` in
  `ocean.workspace.json` / the repo's `ocean.config.json`), not a path. When present, the run
  is scoped to that repo plus the transitive closure of its dependencies, walked in
  topological order so every dependency is refreshed before the repo that needs it. When
  absent, the run covers the whole workspace exactly as before.
- **`--no-deps`** is a modifier on `--repo`: it restricts the run to the named repo alone,
  skipping its dependency tree. It is only meaningful alongside `--repo` — supplied on its
  own it is a usage error (the user has expressed no target to strip dependencies from), and
  the command rejects it rather than silently refreshing the whole workspace.
- **`--clear-hashes`** is the existing flag and composes with scoping: when set, build/check
  hashes are cleared first, and the subsequent refresh — scoped or full — recomputes them.

**Cloning is by registration, not by graph membership.** A targeted repo that is **registered
in `ocean.workspace.json` with a remote** is cloned when missing **even if it declares no
buildable components** — most notably an **app** whose `services`/`libraries`/`testing` are all
empty (a repo that exists upstream but has not yet been broken into Dusk Ocean components).
Such an app is not part of any dependency graph, so it has nothing to install/build/check; the
run clones it and moves on (clone-only). This holds in **both** modes: a whole-workspace
`refresh` clones every registered repo with a remote that is missing (including empty app
shells), and `refresh --repo <app>` clones that app even with no components. A repo with a
remote of `none` (or unset) is skipped with `skipping clone of <name>: no remote configured`.
A `--repo` name that matches **no** registered repo is still the unknown-repo failure.

Scoping also changes one whole-workspace side effect: a full refresh additionally clones the
workspace's **non-code** repositories (infra, docs). A scoped refresh does **not** — those are
not part of any code repo's dependency graph, and pulling them in would defeat the point of
narrowing the run. They remain the full refresh's responsibility.

Re-running is safe: missing repos are cloned, present ones are left in place, and install/
build/check are the same idempotent tasks the full refresh runs (unchanged inputs are skipped
via the hash cache).

### Output & Exit Codes

stdout carries the **progress narrative** — one line per repo as it is cloned, installed,
built, and checked, emitted in the dependency order the run resolves. This is the human-facing
result; it is intentionally the same line shapes the full refresh already prints (`cloning
<name>`, `install skipped for <label>: no install command`, and the like), so scoping changes
*which* repos appear, not the format of each line. There is no machine-readable mode in this
version; a caller should branch on the exit code, not parse stdout.

stderr carries diagnostics and the forwarded stdout/stderr of any install/build/check
subprocess that fails, so a failure's detail is visible without polluting the progress stream.

Exit codes follow the conventional split — `0` means every targeted repo completed all of
clone/install/build/check; any non-zero code means the run did not fully succeed and the
workspace may be partially refreshed. A caller scripting around `refresh` should treat
non-zero as "do not proceed" and read stderr for the cause. The conditions that produce a
non-zero exit are the frontmatter `failure_modes`: a usage error (`--no-deps` without
`--repo`), an unknown `--repo` name, a dependency cycle among the targeted repos, or a failure
in any clone/install/build/check step. This version does not subdivide non-zero into distinct
codes (e.g. a separate code for usage vs. runtime failure); see Decisions.

## Mock

N/A — a cli boundary's mock rung is a no-op (see `.claude/rules/status-gating.md`). This
command has no programmatic consumer that must integrate against it before it is built — its
callers are humans at a terminal and CI steps — so no `command_stub` is generated; the boundary
advances `wire → build` directly. A stub would be optional and is deliberately
skipped here.

## Dependencies

None. The command consumes the workspace config and the clone/install/build/check task
machinery, but those are internal modules of the CLI, not separate specced boundaries.

## Decisions

> **Flag over positional for the target** — *Context*: `refresh` currently takes no positional
> arguments and the target needed a clear, optional surface. *Decision*: name the target with
> `--repo <name>` rather than a positional `refresh <name>`. *Why*: it reads explicitly, keeps
> the bare `refresh` (whole-workspace) default untouched and unambiguous, and matches the
> existing `--clear-hashes` flag style. *Rejected*: an optional positional arg — terser, but it
> blurs the no-arg default with the scoped form and reads less clearly at a glance.

> **Dependencies included by default; `--no-deps` to opt out** — *Context*: a scoped refresh
> has to decide whether "refresh this repo" means the repo alone or the repo plus what it needs
> to build. *Decision*: include the transitive dependency tree by default; `--no-deps` narrows
> to the single repo. *Why*: a repo refreshed without its dependencies usually can't build or
> check, so deps-by-default is the useful, least-surprising case; `--no-deps` stays available
> for the rare "just this one" need. *Rejected*: deps as opt-in (`--with-deps`) — makes the
> common, working case the more verbose one.

> **Single non-zero exit code for v1** — *Context*: failures span usage errors, unknown repos,
> cycles, and step failures, which a richer scheme might map to distinct codes. *Decision*: emit
> `0` for success and a single non-zero for any failure in this version. *Why*: the existing
> `refresh` already collapses failures to a returned error (non-zero); callers today branch on
> zero vs. non-zero, and inventing codes the requester didn't ask for would over-specify the
> contract. *Rejected*: distinct codes (e.g. `2` for usage) — defensible later if a caller needs
> to branch on failure kind, but unjustified now.

> **Clone registered repos by registration, not graph membership (v0.2.0)** — *Context*: a repo
> can be registered in `ocean.workspace.json` with a remote but not yet broken into Dusk Ocean
> components — e.g. an `app` with empty `services`/`libraries`/`testing` (`board_education`).
> The graph-driven clone step only cloned repos that contributed nodes, so such a shell was
> never cloned by either the full or the scoped refresh, and `refresh --repo <it>` reported it
> as having no buildable components. But teams need the repo on disk first in order to *do* that
> componentization. *Decision*: clone any targeted, registered repo that has a remote and is
> missing, regardless of declared components — in both whole-workspace and `--repo` runs; a
> component-less app is clone-only (nothing to install/build/check). *Why*: it matches the
> command's own stated purpose ("clones any that are missing where a remote is configured"), and
> getting the code local is the prerequisite for the manual alignment work. Additive, so a MINOR
> bump (0.1.0 → 0.2.0). *Rejected*: clone-shells only under `--repo` — leaves the whole-workspace
> refresh silently skipping registered apps, which is the more surprising of the two.
