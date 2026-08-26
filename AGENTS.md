# Switch Library Manager

## Project Shape

- This is a cross-platform Go desktop application for scanning and organizing Nintendo Switch backup files.
- The Go module is `src/go.mod`. Run Go commands from `src`, not from the repository root.
- Runtime state is stored beside the executable: `settings.json`, `titles.json`, `versions.json`, `slm.db`, and `slm.log`.
- The application has two modes: an Astilectron GUI and a command-line workflow. Both use the same `db`, `settings`, `switchfs`, and `process` packages.
- `src/resources/app` is the embedded HTML/CSS/JavaScript frontend.

## Important Paths

- `src/main.go`: executable-relative startup, logging, mode selection, and generated asset entrypoints.
- `src/gui.go`: GUI lifecycle, state, frontend message handling, and JSON responses.
- `src/console.go`: CLI workflow, progress output, and CSV export.
- `src/db`: remote title data, local scan model, BoltDB persistence, and scan/cache orchestration.
- `src/switchfs`: binary Switch container parsing and decryption.
- `src/fileio`: split-file metadata dispatch.
- `src/process`: missing-content calculations and file-moving/deletion operations.
- `src/settings`: JSON settings, prod.keys discovery, and update checks.
- `.github`: bundler setup, build, and artifact publication.

## Development Rules

- Inspect `git status` before editing and do not overwrite unrelated worktree changes.
- Keep changes small and preserve the existing package boundaries. Low-level parsers should not gain dependencies on UI code.
- Use explicit errors and context in new code. Do not silently discard errors from file, JSON, network, database, or rename/delete operations.
- Prefer explicit settings, key, logger, and progress dependencies in new code. Existing settings and key state is process-global, so tests must not assume it is reset between cases.
- Treat all file contents, filenames, remote JSON, and settings as untrusted input.
- Organization and cleanup are destructive. Never test them against a real library; use temporary directories and synthetic files.

## Data Invariants

- Title IDs are expected to be 16 hexadecimal characters and are used as lowercase map keys.
- Base title IDs end in `000`; update IDs end in `800`; other IDs are grouped as DLC using the fourth hexadecimal character from the right.
- Local and remote title grouping must use the same normalization and grouping rules.
- One physical NSP/XCI can contain multiple logical content records. Do not assume one file equals one title record.
- `LocalSwitchFilesDB.Skipped` contains diagnostics for unsupported, duplicate, old, malformed, and unrecognized files. Preserve those diagnostics when changing scan behavior.
- `LatestUpdate` means the highest locally observed version, not necessarily the newest remote version.

## Generated Files and Assets

- Do not hand-edit `src/bind_*_amd64.go`, `src/windows.syso`, `src/output`, or `src/astilectron-bundler.exe`; they are generated or build artifacts and are ignored.
- Changes under `src/resources/app` require running the Astilectron bundler before a packaged build. The bundler regenerates platform bindata.
- Keep `src/bundler.json` environment targets synchronized with `.github/actions/publish-artifacts/action.yml`.

## Verification

From `src`:

```text
gofmt -l .
go test ./...
go vet ./...
```

- At the time this guide was created, `go test ./...` and `go vet ./...` fail on the existing vet diagnostic in `switchfs/nca.go` (`int` to `string` conversion). `go test -vet=off ./...` passes the current tests, but does not replace fixing vet.
- For parser, filesystem, persistence, or shared-state changes, add focused tests and run `go test -race ./...` when practical.
- Report whether tests used synthetic fixtures, filename-only fallback data, or real encrypted Switch files. Do not add proprietary keys or large game images to the repository.
- For packaging changes, run `./astilectron-bundler` from `src` and verify every configured output directory. The CI build also runs `go get`, installs the bundler, copies it into `src`, and then invokes it.
