# Embedded Frontend

## Technology and Build

- This is a vanilla JavaScript frontend using jQuery, JsRender, Tabulator, Moment, and Electron's remote APIs through Astilectron.
- There is no frontend package/build pipeline in this repository. Third-party libraries under `lib` are checked-in runtime assets; do not casually replace or regenerate them.
- `app.html` contains the tab shell and JsRender templates, `app.js` contains state and bridge behavior, and `app.css` contains the application layout/style.
- After changing frontend assets, run the bundler from `src` so the embedded platform bindata is regenerated for packaged builds.

## Go Bridge Contract

- The bridge sends `{name, payload}` messages through `astilectron.sendMessage` and receives server messages through `astilectron.onMessage`.
- Keep Go and JavaScript message names, payload encodings, and callback timing synchronized. Most payloads are JSON strings; `isKeysFileAvailable` and `checkUpdate` have special scalar responses.
- Important server-to-client messages are `libraryLoaded`, `missingGames`, `updateProgress`, `error`, and `rescan`.
- The backend serializes GUI state with a mutex, but the frontend also caches `library`, `updates`, `dlc`, and `missingGames`. Clear dependent caches after rescan, settings changes, and organization.

## UI Behavior

- Tabs are rendered lazily. A tab can be requested before its library data exists, so preserve the existing loading/empty/error states.
- File paths are passed to Electron `shell.showItemInFolder`; treat them as filesystem paths, not URLs, and do not normalize them in a way that breaks platform separators.
- Organize is destructive and already asks for confirmation. Keep confirmation and visible progress/error handling when changing that flow.
- Preserve the current layout and styling unless the task is explicitly a UI redesign. Test the fixed navigation/progress area at narrow window sizes as well as the default desktop size.

## Compatibility

- `require('electron').remote` and `EnableRemoteModule` are version-sensitive. Check the Astilectron/Electron versions before changing these APIs.
- Avoid adding a modern browser or Node API without checking the Electron runtime bundled by the current Go dependencies.
- Do not put secrets, prod.keys, or game paths into frontend fixtures or logs.
