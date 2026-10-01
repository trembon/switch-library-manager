# Database and Scanning

## Responsibilities

- `switchTitlesDB.go` converts downloaded title/version JSON into a normalized remote map.
- `localSwitchFilesDB.go` scans configured folders, obtains deep metadata or filename fallback metadata, classifies base/update/DLC files, and records skipped-file diagnostics.
- `persistentDB.go` wraps BoltDB. Runtime database path is `<baseFolder>/slm.db`.
- `utils.go` downloads and validates the remote JSON files, using ETags and a local fallback.

## Models and Classification

- `LocalSwitchFilesDB.TitlesMap` is keyed by a normalized title prefix and contains one base, version-keyed updates, and title-ID-keyed DLC records.
- `Skipped` is keyed by `ExtendedFileInfo`; it is the user-visible explanation for unsupported, duplicate, old, malformed, and unrecognized files.
- Duplicate base/update/DLC and older local versions are not automatically deleted during scanning. Deletion is a later `process` operation.
- Local grouping and remote grouping duplicate title-ID rules. Any change to one must update the other or centralize the rule and add regression tests.
- Filename fallback provides only title ID and numeric version. Name, region, NACP, and other deep metadata may be nil.

## Cache and Persistence

- BoltDB buckets currently include `internal-metadata`, `local-library`, and `deep-scan`. Local records use keys `files`, `skipped`, and `titles`.
- Deep-scan cache keys are based on `filePath|fileName|fileSize`. Same-name/same-size replacement can therefore retain stale metadata; do not weaken invalidation without considering this limitation.
- Values are encoded with Go `encoding/gob`. Changing persisted struct fields or types can affect existing `slm.db` files. Add a version/migration strategy before making incompatible changes.
- Database transactions must match their operation: bucket creation and writes require `Update`, reads require `View`.
- Use explicit `time.Duration` units for BoltDB timeouts, propagate open/transaction errors, and do not terminate the process from this package.
- Tests must use a temporary base folder, close the database, reopen it, and verify persistence and cache clearing.

## Remote Data

- Remote title and version data is external input. Validate HTTP status, response shape, IDs, and versions before replacing local files.
- Preserve the local file fallback when a remote request fails, but return a useful error when no valid local file exists.
- Close opened files and response bodies. Prefer bounded request timeouts and atomic replacement for future download changes.

## Settings Dependency

`processLocalFiles` currently reads settings through the global singleton, including `ReadSettings("")`. This is an implicit initialization requirement. New code should pass the relevant settings or scan options explicitly, and tests should not rely on test order to initialize settings.
