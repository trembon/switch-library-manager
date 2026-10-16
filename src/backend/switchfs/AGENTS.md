# Switch File Parsers

## Scope

This package parses untrusted NSP/XCI container data, CNMT/NACP metadata, RomFS/PFS0 structures, and NCA headers/sections. It also provides the split-file reader. Inputs may be truncated, malformed, encrypted differently, or deliberately adversarial.

## Safety Requirements

- Every offset, length, count, multiplication, and slice boundary must be validated before indexing or allocating.
- Malformed input must return a descriptive error, not panic. Check both the declared container range and the underlying `io.ReaderAt` result.
- Validate arithmetic for overflow before converting between `uint64`, `int64`, `int`, and `uint32`.
- Do not trust file extensions or names as proof of format. Filename fallback belongs in `db`; parser dispatch must still validate headers.
- Keep file handles closable and avoid loading unbounded external sections into memory without a size check.

## Format Flow

- NSP: top-level PFS0 -> `cnmt.nca` -> decrypted NCA data section -> inner PFS0 -> CNMT -> optional `control.nacp`.
- XCI: XCI header at offset `0x100` -> root HFS0/PFS0 -> `secure` partition -> `cnmt.nca` -> CNMT -> optional NACP.
- NCA parsing currently supports a limited no-rights-ID encryption path and reads keys through the global `settings` key store. Do not claim broader encryption support without fixtures and tests.
- `fileio.ReadSplitFileMetadata` tries normal NSP/PFS0 parsing and then XCI parsing over `switchfs.OpenFile`.

## Metadata Invariants

- CNMT title IDs must be normalized consistently and should be exactly 16 lowercase hexadecimal characters before they become map keys.
- `ContentMetaAttributes.Ncap` is optional. Code must handle filename-only metadata and DLC metadata without NACP.
- A multi-content container can produce multiple metadata entries from one physical file.
- Keep crypto implementation changes isolated and test key derivation, counter construction, and decryption against known-good fixtures without committing keys.

## Split Files

- Split detection relies on numeric filename suffixes and directory contents. Validate an empty directory, missing parts, ordering, chunk sizes, and reads that cross chunk boundaries.
- Check `part >= len(parts)`, not only `part > len(parts)`.
- Check errors when opening each chunk. Never dereference a nil file after a failed open.
- Close every lazily opened chunk on all paths.

## Tests

Prefer table-driven unit tests with byte slices or temporary files for:

- Valid and truncated PFS0/HFS0 headers.
- CNMT/NACP short buffers, invalid counts, invalid offsets, and title-ID formatting.
- NCA section bounds and unsupported encryption/hash types.
- Split ordering, boundary reads, missing chunks, empty directories, and close behavior.
- Filename-only metadata passed into higher-level code.

Use real encrypted files only when necessary, document the fixture assumptions, and never commit `prod.keys` or game content.
