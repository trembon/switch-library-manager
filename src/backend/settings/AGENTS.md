# Settings and Keys

## Runtime State

- `settings.json`, `titles.json`, and `versions.json` live beside the executable, using the `baseFolder` selected by `main.go`.
- Defaults are created on first launch. Settings also retain remote ETags, scan folders, ignore lists, GUI behavior, and organization templates.
- `ReadSettings` and `SaveSettings` currently update process-global `settingsInstance`; a later base-folder argument is ignored after the first read.
- `SwitchKeys` and `InitSwitchKeys` likewise use process-global key state. Do not write tests that assume a clean singleton or that changing settings automatically reloads keys.

## Configuration Contract

- Preserve the JSON field names in `AppSettings` and `OrganizeOptions`; the GUI displays and sends this structure directly.
- Treat settings files as untrusted input. Validate JSON, paths, URLs, numeric page sizes, title IDs, ignore lists, and organization templates before use.
- Do not silently accept malformed JSON or silently discard save errors. Prefer explicit errors and atomic replacement when changing persistence code.
- The README settings example contains explanatory comments and is not valid JSON. Use actual JSON in fixtures and tests.

## Key Discovery

`InitSwitchKeys` searches in this order:

1. The configured `prod_keys` path, treating a non-`.keys` path as a directory.
2. `<baseFolder>/prod.keys`.
3. The documented home-directory location.

The application needs `header_key` and application key-area keys for deep metadata scans. Without keys, `db` falls back to title ID/version tags in filenames. Never commit keys, key-derived secrets, or real encrypted game files.

## Network and Updates

- `CheckForUpdates` reads the remote version JSON and compares it with `SLM_VERSION`.
- Remote title/version files are downloaded by `db.LoadAndUpdateFile`; preserve a valid local file when the network is unavailable.
- Use bounded HTTP requests, check status codes and required fields, close response bodies, and validate downloaded JSON before replacing local state.
- Tests should use an `httptest` server or local readers, not the live title databases.

## Tests

Add coverage for first-run defaults, malformed/truncated settings, missing local data, ETag behavior, key search order, missing keys, custom key paths, and isolation between temporary base folders. Tests must not depend on execution order or global state left by another test.
