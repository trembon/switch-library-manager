# Settings

Switch Library Manager uses a categorized `settings.json` format. The file is valid JSON and must contain the current `"schema_version"` value, which is currently `2`.

When using the graphical interface, the Settings tab provides a structured editor for these options. The schema version and internal cache values are not shown. Saving settings writes the JSON file and displays a restart notice; the existing folder picker remains an immediate path-and-rescan workflow.

## Example

```json
{
  "schema_version": 2,
  "gui": {
    "enabled": true,
    "page_size": 100,
    "hide_missing_games": false,
    "hide_demo_games": false,
    "theme": "inherit"
  },
  "paths": {
    "library_folder": "",
    "scan_folders": [],
    "prod_keys": ""
  },
  "scan": {
    "recursive": true,
    "ignore_file_types": []
  },
  "organization": {
    "create_folder_per_game": false,
    "dlc_folder": "",
    "updates_folder": "",
    "rename_files": false,
    "delete_empty_folders": false,
    "delete_old_update_files": false,
    "folder_name_template": "{TITLE_NAME}",
    "switch_safe_file_names": true,
    "file_name_template": "{TITLE_NAME} ({DLC_NAME})[{TITLE_ID}][v{VERSION}]",
    "process_when_missing_base_game": false
  },
  "missing_content": {
    "check_for_updates": true,
    "check_for_dlc": true,
    "ignore_dlc_updates": false,
    "ignore_dlc_title_ids": ["01007F600B135007"],
    "ignore_update_title_ids": []
  },
  "data_sources": {
    "titles_url": "https://tinfoil.io/repo/db/titles.json",
    "versions_url": "https://raw.githubusercontent.com/blawar/titledb/master/versions.json"
  },
  "logging": {
    "debug": false
  }
}
```

## Settings reference

| Setting | Type | Default | Purpose |
| --- | --- | --- | --- |
| `schema_version` | integer | `2` | Identifies the settings format. It is required and is not an operational preference. |
| `gui.enabled` | boolean | `true` | Starts the graphical interface when true. The `-m` command-line flag overrides it. |
| `gui.page_size` | integer | `100` | Number of rows shown per page in GUI tables. Values less than or equal to zero are reset to 100. |
| `gui.hide_missing_games` | boolean | `false` | Hides the missing-games tab in the GUI. |
| `gui.hide_demo_games` | boolean | `false` | Excludes demo titles from the GUI missing-games list. |
| `gui.theme` | string | `"inherit"` | GUI color mode: `inherit` follows the operating system, while `light` and `dark` force a mode. Invalid or missing values use `inherit`. |
| `paths.library_folder` | string | `""` | Main folder scanned by the GUI and console workflow. The `-f` flag overrides it for a console run. |
| `paths.scan_folders` | string array | `[]` | Additional folders scanned alongside the main library folder. |
| `paths.prod_keys` | string | `""` | Optional file or directory containing `prod.keys`. If it does not resolve, the application checks the executable folder and the home `.switch` folder. |
| `scan.recursive` | boolean | `true` | Scans subdirectories. The `-r` flag overrides it for a console run. |
| `scan.ignore_file_types` | string array | `[]` | File extensions ignored when reporting unsupported file types. Extensions may include or omit the leading dot. |
| `organization.create_folder_per_game` | boolean | `false` | Creates a folder for each game during organization. |
| `organization.dlc_folder` | string | `""` | Optional subfolder for DLC files. |
| `organization.updates_folder` | string | `""` | Optional subfolder for update files. |
| `organization.rename_files` | boolean | `false` | Renames files using `file_name_template`. |
| `organization.delete_empty_folders` | boolean | `false` | Removes empty folders after old updates are deleted. |
| `organization.delete_old_update_files` | boolean | `false` | Deletes duplicate and old update files during the workflow. |
| `organization.folder_name_template` | string | `"{TITLE_NAME}"` | Template used for game folder names. |
| `organization.switch_safe_file_names` | boolean | `true` | Replaces characters that are unsafe for Switch-compatible names. |
| `organization.file_name_template` | string | `"{TITLE_NAME} ({DLC_NAME})[{TITLE_ID}][v{VERSION}]"` | Template used when renaming files. |
| `organization.process_when_missing_base_game` | boolean | `false` | Allows updates and DLC to be organized even when the base game is absent. |
| `missing_content.check_for_updates` | boolean | `true` | Runs the missing-updates check. |
| `missing_content.check_for_dlc` | boolean | `true` | Runs the missing-DLC check. |
| `missing_content.ignore_dlc_updates` | boolean | `false` | Excludes DLC updates from the missing-updates calculation. |
| `missing_content.ignore_dlc_title_ids` | string array | `["01007F600B135007"]` | DLC title IDs excluded from the missing-DLC calculation. IDs are compared case-insensitively. |
| `missing_content.ignore_update_title_ids` | string array | `[]` | Update title IDs excluded from the missing-updates calculation. IDs are compared case-insensitively. |
| `data_sources.titles_url` | string | Tinfoil titles URL | URL used to download the title metadata database. |
| `data_sources.versions_url` | string | Blawar versions URL | URL used to download the version metadata database. |
| `logging.debug` | boolean | `false` | Enables debug-level logging in `slm.log`. |

## Organization templates

The following tokens are available in folder and file templates:

- `{TITLE_NAME}`: game name.
- `{TITLE_ID}`: title ID.
- `{VERSION}`: numeric content version.
- `{VERSION_TXT}`: display version such as `1.0.0`.
- `{REGION}`: title region.
- `{TYPE}`: `BASE`, `UPD`, or `DLC` content type.
- `{DLC_NAME}`: DLC name.

When `rename_files` is enabled, `file_name_template` must contain `{TITLE_NAME}` or `{TITLE_ID}`. When `create_folder_per_game` is enabled, `folder_name_template` must contain `{TITLE_NAME}` or `{TITLE_ID}`.

## Internal cache

`cache.json` stores the HTTP ETags used for conditional downloads:

```json
{
  "titles_etag": "...",
  "versions_etag": "..."
}
```

ETags are runtime cache state rather than user preferences, so they are intentionally excluded from `settings.json`. The cache is recreated with default ETags when absent.

## Migrating older settings

On startup, the application checks the `schema_version` in `settings.json`. If the marker is missing or lower than the current schema, the file is preserved by renaming it to `settings.old.json`. If that name already exists, it uses `settings.old.1.json`, `settings.old.2.json`, and so on. A new `settings.json` containing current default values is then created.

Older settings are not converted automatically. Copy values into the generated current settings file using this mapping for the original settings format:

| Older setting | Current setting |
| --- | --- |
| `gui` | `gui.enabled` |
| `gui_page_size` | `gui.page_size` |
| `hide_missing_games` | `gui.hide_missing_games` |
| `hide_demo_games` | `gui.hide_demo_games` |
| `folder` | `paths.library_folder` |
| `scan_folders` | `paths.scan_folders` |
| `prod_keys` | `paths.prod_keys` |
| `scan_recursively` | `scan.recursive` |
| `ignore_file_types` | `scan.ignore_file_types` |
| `organize_options.*` | `organization.*` |
| `check_for_missing_updates` | `missing_content.check_for_updates` |
| `check_for_missing_dlc` | `missing_content.check_for_dlc` |
| `ignore_dlc_updates` | `missing_content.ignore_dlc_updates` |
| `ignore_dlc_title_ids` | `missing_content.ignore_dlc_title_ids` |
| `ignore_update_title_ids` | `missing_content.ignore_update_title_ids` |
| `titles_json_url` | `data_sources.titles_url` |
| `versions_json_url` | `data_sources.versions_url` |
| `debug` | `logging.debug` |

The older `titles_etag` and `versions_etag` values are not copied into `settings.json`; they belong in `cache.json`. If they are omitted, the application will rebuild the cache and refresh the local metadata as needed.

Set `schema_version` to the current value, `2`. A file marked with a future schema version or containing malformed JSON is rejected without being overwritten so that it can be migrated manually.

The recognized older format is a valid JSON object with no `schema_version` or with a lower value. A non-object JSON value and unsupported future schema versions remain in place and stop startup. Console mode also stops after an older-settings migration so it cannot run scans or destructive organization using unreviewed defaults; use the GUI or edit the generated file first.
