# Library Processing

## Destructive Behavior

`OrganizeByFolders` renames and moves library files. `DeleteOldUpdates` removes files marked as old local updates and may remove empty directories. Treat both as destructive APIs:

- Never run them against a real user library during development or tests.
- Use temporary directories with synthetic files and assert every source, destination, and deletion.
- Check destination collisions, path roots, missing files, permissions, and rerun behavior.
- Report move/delete errors rather than treating a partial operation as success.
- Refresh or invalidate the local scan model after successful moves; stored paths otherwise become stale.

## Ordering and Callers

- The console currently deletes old updates before organization.
- The GUI currently organizes first and then calls `DeleteOldUpdates` using the old scan paths. Preserve or fix this difference deliberately, and test both entrypoints before changing ordering.
- `ProcessWhenMissingBaseGame` is a special path. Do not assume `v.File` is a valid base record when a title has only updates or DLC.
- Multi-content files can appear in base, update, and DLC records while referring to the same physical path. Avoid moving the same file more than once.

## Templates and Paths

- Supported placeholders are `{TITLE_NAME}`, `{TITLE_ID}`, `{VERSION}`, `{VERSION_TXT}`, `{REGION}`, `{TYPE}`, and `{DLC_NAME}`.
- Template output becomes a filesystem path after safe-name conversion. Validate empty output, illegal separators, traversal, reserved names, and collisions.
- `UpdatesFolder` and `DlcFolder` need explicit root semantics. Do not assume a relative destination is relative to the configured library folder unless the code makes that explicit.
- Safe-name transliteration can collapse different titles to the same name. Test collision numbering and pre-existing destination files.
- `Metadata` and `Metadata.Ncap` are independently optional. Filename fallback and DLC records commonly have no NACP; guard both levels.

## Missing Content

- `ScanForMissingUpdates` compares the highest local update against the highest remote update and can include DLC updates unless configured otherwise.
- `ScanForMissingDLC` compares remote DLC IDs with local DLC IDs.
- Both functions use maps, so output ordering is not stable. Sort at the presentation boundary if deterministic CLI, GUI, or CSV output is required.
- Apply ignore-title settings after normalizing IDs consistently with database classification.

## Tests

Add focused tests for template replacement, repeated placeholders, safe names, base/update/DLC organization, missing-base processing, multi-content files, duplicate destinations, old-update deletion, empty-folder cleanup, and both CLI/GUI ordering assumptions.
