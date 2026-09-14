# CI and Packaging

## Workflows

- `workflows/verify-pr.yml` checks out the repository, runs `actions/setup-environment`, and invokes `actions/build-app`.
- `workflows/build-master.yml` performs the same setup/build and then publishes Windows, Linux, and macOS artifacts.
- Composite actions run on Ubuntu and use Bash commands with `working-directory: ./src`.

## Build Contract

- CI uses Go `1.27`, installs the pinned `github.com/asticode/go-astilectron-bundler/astilectron-bundler@v0.7.12`, copies it to `src`, and runs `./astilectron-bundler`.
- The bundler generates ignored platform bindata files and writes outputs under `src/output`.
- Any change to `src/bundler.json` must be checked against artifact paths in `actions/publish-artifacts/action.yml`.
- The release action currently requires all three paths: `src/output/windows-amd64/*`, `src/output/linux-amd64/*`, and `src/output/darwin-amd64/*`. Restricting bundler environments without updating publication will make the release fail.

## Change Rules

- Do not commit generated outputs, the local bundler executable, embedded bindata, or platform-specific local build products.
- Keep setup and build commands reproducible; prefer pinned or module-resolved versions when changing dependencies.
- If adding tests or vet checks, ensure they run from `src` because the repository root is not a Go module.
- Every PR verification must run `go test ./...` from `src` before the build step.
- `workflows/verify-pr.yml` must analyze aggregate statement coverage for `db`, `fileio`, `process`, `settings`, `switchfs`, and `switchfs/_crypto`, and fail when coverage is below 90%.
- For workflow edits, validate YAML structure and reason about missing artifact behavior (`if-no-files-found: error`) before merging.
