# CI and Packaging

## Workflows

- `workflows/verify-pr.yml` checks out the repository, runs `actions/setup-environment`, and invokes `actions/build-app`.
- `workflows/build-master.yml` performs the same setup/build and then publishes Windows, Linux, and macOS artifacts.
- Composite actions use Bash commands with `working-directory: ./src`; the release workflow runs the build on native Windows, Linux, and macOS runners.

## Build Contract

- CI uses Go `1.27`, installs the pinned Wails CLI, generates module bindings, and runs `wails build`.
- Wails writes platform build products under `src/build/bin`; these are not committed.
- Platform targets in `workflows/build-master.yml` must stay synchronized with artifact names in `actions/publish-artifacts/action.yml`.
- Release builds run natively on Windows, Linux, and macOS because Wails desktop builds require platform toolchains and WebView dependencies.

## Change Rules

- Do not commit generated build outputs or platform-specific local build products. Commit Wails JavaScript bindings because this frontend has no package-install/build step.
- Keep setup and build commands reproducible; prefer pinned or module-resolved versions when changing dependencies.
- If adding tests or vet checks, ensure they run from `src` because the repository root is not a Go module.
- Every PR verification must run `go test ./...` from `src` before the build step.
- `workflows/verify-pr.yml` must analyze aggregate statement coverage for `db`, `fileio`, `process`, `settings`, `switchfs`, and `switchfs/_crypto`, and fail when coverage is below 90%.
- For workflow edits, validate YAML structure and reason about missing artifact behavior (`if-no-files-found: error`) before merging.
