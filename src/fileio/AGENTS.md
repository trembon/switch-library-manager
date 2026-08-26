# File I/O Dispatch

- `ReadSplitFileMetadata` is the adapter between the scanner and `switchfs`; keep format dispatch here rather than adding scanner-specific parser logic.
- Split files are opened through `switchfs.OpenFile`, which returns an `io.ReaderAt`/`io.Closer` abstraction over either one file or multiple numbered chunks.
- Preserve close behavior on every success and error path. A split reader may lazily open several chunk handles.
- Do not treat a successful dispatch as proof that metadata is valid. NSP/XCI parsing must validate its own headers and bounds and return errors for malformed input.
- Tests should use temporary directories and synthetic files for ordinary and split NSP/XCI dispatch, missing parts, empty directories, malformed headers, and cleanup after errors.
- Do not add keys, encrypted game images, or proprietary fixtures to this package.
