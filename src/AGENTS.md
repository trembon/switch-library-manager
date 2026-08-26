# Go Application Code

## Entry Points and Flow

- `main.go` resolves the executable directory and uses it as the application base folder. It is not the current working directory. macOS app bundles have special path handling.
- Startup loads settings, creates `slm.log`, initializes CLI flags, and selects GUI or console mode. `-m console` and `-m gui` override `settings.json`.
- The console flow downloads/loads remote titles and versions, scans the library, reports issues, optionally deletes old updates, organizes files, and checks missing updates/DLC.
- The GUI flow starts Astilectron, loads `resources/app/app.html`, and exchanges messages with `app.js`. It maintains mutable local and remote DB state in `GUI.state`.
- `db.ProgressUpdater` is the progress callback used by both CLI and GUI. Do not assume progress totals are always nonzero or that callbacks are synchronous unless the caller guarantees it.

## Package Boundaries

- `main` owns orchestration and presentation adapters.
- `db` owns remote/local models, scanning, cache reads/writes, and persistence.
- `switchfs` owns parsing and decryption of external binary formats.
- `fileio` adapts split files to the parser layer.
- `process` owns content comparisons and destructive organization.
- `settings` owns JSON configuration and key discovery.
- `console` owns flags and Windows console attachment.

When adding reusable logic, put it in the narrowest package that owns the behavior. Do not put parser, database, or file-organization logic in `gui.go` or `console.go` merely because those files call it.

## Shared State and JSON Contracts

- `settings.ReadSettings` and `settings.SwitchKeys` currently use process-global singletons. A later `baseFolder` is ignored after initialization. Tests that need isolation must use separate processes or first refactor toward explicit state.
- GUI message names and payload shapes are an API between `gui.go` and `resources/app/app.js`. Update both sides together and preserve the asynchronous callback behavior.
- Important messages include `loadSettings`, `saveSettings`, `isKeysFileAvailable`, `checkUpdate`, `updateDB`, `updateLocalLibrary`, `missingGames`, `missingUpdates`, `missingDlc`, `organize`, `rescan`, `libraryLoaded`, `updateProgress`, and `error`.
- Keep JSON field names stable unless a coordinated frontend change or migration is included.

## Error and Concurrency Rules

- Return errors from constructors and operations rather than calling `log.Fatal` or panicking in package code.
- Do not ignore errors from BoltDB, file reads/writes, JSON decoding, `os.Rename`, `os.Remove`, or frontend message serialization.
- GUI operations are serialized through `State.Mutex`, but maps, settings, keys, and the database still have shared mutable state. Recheck locking before adding asynchronous work.
- Do not use `os.Executable` or executable-relative runtime files as implicit test fixtures. Tests should pass explicit temporary directories.

## Commands

```text
cd src
gofmt -w <changed-go-files>
go test ./...
go vet ./...
```

The repository currently has very limited tests. A successful `go test -vet=off ./...` is only a compile/basic-test signal until parser, cache, and organization behavior has focused coverage.
