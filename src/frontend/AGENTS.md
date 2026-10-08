# Embedded Frontend

## Technology and Build

- This is a vanilla JavaScript frontend using jQuery, JsRender, Tabulator, Moment, and Wails-generated Go bindings.
- There is no frontend package/build pipeline in this repository. Third-party libraries under `lib` are checked-in runtime assets; do not casually replace or regenerate them.
- `index.html` contains the tab shell and JsRender templates, `app.js` contains state and Wails binding/event behavior, and `app.css` contains the application layout/style.
- After changing Go bindings, run `wails generate module` from `src`. `wails build` embeds the frontend assets for packaged builds.

## Go Bridge Contract

- Go methods are imported from `wailsjs/go/app/App.js`, and runtime events are imported from `wailsjs/runtime/runtime.js`.
- Keep Wails method signatures, event names, payload types, and callback timing synchronized. Bindings use typed values rather than JSON bridge messages.
- Important server-to-client events are `updateProgress`, `error`, and `rescan`.
- The backend serializes GUI state with a mutex, but the frontend also caches `library`, `updates`, `dlc`, and `missingGames`. Clear dependent caches after rescan, settings changes, and organization.

## UI Behavior

- Tabs are rendered lazily. A tab can be requested before its library data exists, so preserve the existing loading/empty/error states.
- File paths are passed to the Go `ShowInFolder` binding; treat them as filesystem paths, not URLs, and do not normalize them in a way that breaks platform separators.
- Organize is destructive and already asks for confirmation. Keep confirmation and visible progress/error handling when changing that flow.
- Preserve the current layout and styling unless the task is explicitly a UI redesign. Test the fixed navigation/progress area at narrow window sizes as well as the default desktop size.

## Compatibility

- Avoid adding browser or Node APIs when a Wails runtime or Go binding is more appropriate.
- Do not put secrets, prod.keys, or game paths into frontend fixtures or logs.
