# Version 2 beta is available

To clean up this application and make it a bit more modern, version 2 beta is currently available with the migrations to a different build platform (astilectron to wails).
Wails is under active development and will also result in better performance and smaller build sizes.

With this I am also doing a lot of the changes I have wanted under the years.

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
You can also specify a custom location in [`settings.json`](docs/settings.md).

Note: Only the header_key, and the key_area_key_application_XX keys are required.

## Settings

The application creates a current `settings.json` on first launch. You can customize paths, scanning, organization, missing-content checks, and GUI behavior there. See the [complete settings reference](docs/settings.md) for the format, cache behavior, and migration instructions, and the [naming template guide](docs/naming.md) for file and folder naming options.

## Usage

### Windows

- Extract the zip file
- Double click the Exe file
- If you want to use command line mode, update `settings.json` with `"gui": {"enabled": false}`, or pass `-m console`
  - Open `cmd`
  - Run `switch-library-manager.exe`

### macOS

- Extract the zip file
- Double click the App file
- If you want to use command line mode, update `settings.json` with `"gui": {"enabled": false}`, or pass `-m console`
  - Open your Terminal
  - `cd` to the folder containing `switch-library-manager`
  - `chmod +x switch-library-manager` to make it executable
  - Run `./switch-library-manager`

### Linux

- Choose the `linux-amd64-webkit41` download when your distribution provides WebKitGTK 4.1, including CachyOS and Ubuntu 24.04. The WebKitGTK 4.0 download is a compatibility option for systems that provide 4.0 but not 4.1.
- Install the matching WebKitGTK runtime library and GTK 3. Ubuntu 22.04 examples: `sudo apt install libwebkit2gtk-4.0-37` or `sudo apt install libwebkit2gtk-4.1-0`. These packages bring in their runtime dependencies, including GTK 3.
- Extract the archive, open a terminal in its folder, and run `chmod +x switch-library-manager` followed by `./switch-library-manager`.
- The executable also runs console mode (`./switch-library-manager -m console`) and has the same WebKitGTK runtime requirement.

The release archives are named `switch-library-manager-v<VERSION>.linux-amd64-webkit40.tar.gz` and `switch-library-manager-v<VERSION>.linux-amd64-webkit41.tar.gz`; the suffix identifies the required WebKitGTK API version.

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

#### Linux WebKitGTK variants

From the `src` directory, install the build dependencies matching the selected version. For WebKitGTK 4.0, use `libgtk-3-dev` and `libwebkit2gtk-4.0-dev`, then run `wails generate module` and `wails build -platform linux/amd64 -webview2 embed`. For WebKitGTK 4.1, use `libgtk-3-dev` and `libwebkit2gtk-4.1-dev`, then run `wails generate module -tags webkit2_41` and `wails build -platform linux/amd64 -webview2 embed -tags webkit2_41`.

### Visual Studio Code debugging

- Install the Go and Wails extensions and ensure `wails` is available on `PATH`.
- Open the repository root in VS Code and press `F5`.
- Select `Wails: Debug Switch Library Manager`. The launch configuration builds with debug symbols and starts the combined executable in GUI mode.

The same executable still supports the console workflow. Run `src/build/bin/switch-library-manager.exe -m console` on Windows, or `./src/build/bin/switch-library-manager -m console` on macOS/Linux.

## Thanks

This program relies on [blawar's titledb](https://github.com/blawar/titledb), to get the latest titles and versions.
