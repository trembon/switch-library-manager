# Update 2026-01-06

As seen there havent been much activity in this repo lately which is because the motivation from my side has been low as I dont use this application that much anymore.

I have no problem to continue to keep this repo alive, with viewing/closing issues/pull requests and creating releases, but I have seen some forks created with some continued work that maybe will get more active by time.

# Switch library manager

Fork of [Switch Library Manager](https://github.com/giwty/switch-library-manager) created by giwty with continued improvements and changes

Easily manage your switch game backups

![Image description](https://raw.githubusercontent.com/trembon/switch-library-manager/master/.github/readme/updates_ui.png)

![Image description](https://raw.githubusercontent.com/trembon/switch-library-manager/master/.github/readme/dlc_ui.png)

![Image description](https://raw.githubusercontent.com/trembon/switch-library-manager/master/.github/readme/cmd.png)

## Features

- Cross platform, works on Windows / Mac / Linux
- GUI and command line interfaces
- Scan your local switch backup library (NSP/NSZ/XCI)
- Read titleId/version by decrypting NSP/XCI/NSZ (requires prod.keys)
- If no prod.keys present, fallback to read titleId/version by parsing file name (example: `Super Mario Odyssey [0100000000010000][v0].nsp`).
- Lists missing update files (for games and DLC)
- Lists missing DLCs
- Automatically organize games per folder
- Rename files based on metadata read from NSP
- Delete old update files (in case you have multiple update files for the same game, only the latest will remain)
- Delete empty folders
- Zero dependencies, all crypto operations implemented in Go

## Keys (optional)

Having a prod.keys file will allow you to ensure the files you have a correctly classified.
The app will look for the "prod.keys" file in the app folder or under ${HOME}/.switch/
You can also specify a custom location in the settings.json (see below)

Note: Only the header_key, and the key_area_key_application_XX keys are required.

## Settings

During the App first launch a v2 `settings.json` file will be created, which allows granular control over the app. See [docs/settings-v2.md](docs/settings-v2.md) for the complete reference and v1 migration guide.

You can customize folder/file renaming, enable or disable features, and set title IDs of DLC or updates to ignore.

```json
{
 "schema_version": 2,
 "gui": {
  "enabled": true,
  "page_size": 100,
  "hide_missing_games": false,
  "hide_demo_games": false
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

ETags are stored in the internal `cache.json` file and are not part of user settings.

## Naming template

The following template elements are supported:

- {TITLE_NAME} - game name
- {TITLE_ID} - title id
- {VERSION} - version id (only applicable to files)
- {VERSION_TXT} - version number (like 1.0.0) (only applicable to files)
- {REGION} - region
- {TYPE} - impacts DLCs/updates, will appear as ["UPD","DLC"]
- {DLC_NAME} - DLC name (only applicable to DLCs)

## Usage

### Windows

- Extract the zip file
- Double click the Exe file
- If you want to use command line mode, update `settings.json` with `"gui": {"enabled": false}`, or pass `-m console`
  - Open `cmd`
  - Run `switch-library-manager.exe`

### macOS or Linux

- Extract the zip file
- Double click the App file
- If you want to use command line mode, update `settings.json` with `"gui": {"enabled": false}`, or pass `-m console`
  - Open your Terminal
  - `cd` to the folder containing `switch-library-manager`
  - `chmod +x switch-library-manager` to make it executable
  - Run `./switch-library-manager'

### Console parameters

NOTE: parameters are only usable in command line mode, except the parameter -m (mode) which will override `gui.enabled`.

| Name           | Flag | Value       | Description                                                                                          |
| -------------- | ---- | ----------- | ---------------------------------------------------------------------------------------------------- |
| Mode           | -m   | console/gui | Which mode to start the application in, overrides **gui.enabled** in settings.json                 |
| NSP Folder     | -    | _path_      | Path to the NSP folder, overrides **paths.library_folder** in settings.json                        |
| Recursive scan | -r   | true/false  | If recursive scan should be used for the NSP folder, overrides **scan.recursive** in settings.json |
| Export CSV     | -e   | _path_      | Which folder to output missing_updates, missing_dlcs and issues in CSV format                       |

## Building

### Source layout

- `src/frontend` contains the embedded vanilla JavaScript frontend and generated Wails bindings.
- `src/backend/app` contains Wails GUI orchestration and frontend DTOs.
- `src/backend/consoleapp` contains the command-line workflow.
- `src/backend/db`, `fileio`, `process`, `settings`, and `switchfs` contain reusable domain logic.
- `src/assets/icons` contains source application icons; Wails build assets are generated under `src/build`.

### Windows, macOS, or Linux

- Install and set up Go and the Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0`
- Clone the repo: `git clone https://github.com/trembon/switch-library-manager.git`
- From the repository root, prepare the Wails build assets:
  - PowerShell: `pwsh -File .\scripts\prepare-wails-build-assets.ps1`
  - Bash/Git Bash: `bash scripts/prepare-wails-build-assets.sh`
- Move into the Go/Wails project: `cd switch-library-manager/src`
- Generate the Wails bindings: `wails generate module`
- Build the application: `wails build`
- Binaries will be available under `src/build/bin`

### Visual Studio Code debugging

- Install the Go and Wails extensions and ensure `wails` is available on `PATH`.
- Open the repository root in VS Code and press `F5`.
- Select `Wails: Debug Switch Library Manager`. The launch configuration builds with debug symbols and starts the combined executable in GUI mode.

The same executable still supports the console workflow. Run `src/build/bin/switch-library-manager.exe -m console` on Windows, or `./src/build/bin/switch-library-manager -m console` on macOS/Linux.

## Thanks

This program relies on [blawar's titledb](https://github.com/blawar/titledb), to get the latest titles and versions.
