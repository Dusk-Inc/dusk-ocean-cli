## Developer Config Files — No Silent Defaults

When a developer-managed config file (e.g., `.oceanignore`, `.oceaninclude`) is absent, log the absence and proceed with no-op behavior rather than substituting built-in defaults. Silent defaults hide misconfiguration and make behavior unpredictable; an explicit log message makes the missing file discoverable.

## Test Expected Paths Must Be Hardcoded, Not Derived From the Function Under Test

When testing that output lands in a specific directory (e.g., `.ocean/results/`), assert against a hardcoded expected path rather than calling the function under test to produce the expected value. Deriving the expected path from the same function masks path-construction bugs — the test will pass even when the function returns the wrong directory.

## Update All Call Sites When Extracting a Shared Function

When refactoring command logic into a shared function (e.g. `ContainService`), all call sites — including interactive menu wrappers — must be updated to use the new function. Leaving a menu wrapper with its own inline implementation creates a divergence where the flag-based path and the menu path behave differently.

## Workspace Type Definitions Are Duplicated Across Packages

`WorkspaceService`, `RepoConfig`, and related structs are defined in both `src/functions/workspace.go` and `src/models/workspace.go`. The `functions` package uses its own copies and does not import from `models`. When adding fields to any shared workspace type, both files must be updated — failing to update the functions copy produces a compile error only in files that use the functions package, which can appear as a stale IDE diagnostic.

## Version Bumping

When any change is made to the Dusk Ocean application, increment the version string in `src/functions/config.go`. Shipping code without a version bump makes it impossible to distinguish deployed builds and breaks any downstream tooling that relies on version identity.
