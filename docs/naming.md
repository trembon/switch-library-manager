# Naming templates

Switch Library Manager can create a folder for each game and can rename files while organizing a library. Set `organization.create_folder_per_game` and `organization.rename_files` to enable those behaviors. The folder template is `organization.folder_name_template`; the file template is `organization.file_name_template`.

Templates use tokens written in braces. A token is replaced wherever it appears. The defaults are `{TITLE_NAME}` for folders and `{TITLE_NAME} ({DLC_NAME})[{TITLE_ID}][v{VERSION}]` for files. Size tokens are optional; the defaults do not add a size to names.

## Tokens

| Token | Folder template | File template | Value |
| ----- | -------------- | ------------- | ----- |
| `{TITLE_NAME}` | Yes | Yes | Game name from title data, NACP metadata, or filename fallback. |
| `{TITLE_ID}` | Yes | Yes | Title ID, written in uppercase. |
| `{VERSION}` | Yes | Yes | Numeric version. Update and DLC file names use that record's version. A base in a multi-content file and its game folder use the highest local update version when present; otherwise they use `0`. |
| `{VERSION_TXT}` | Yes | Yes | Display version such as `1.0.0`, when available. A multi-content base and its folder use the display version from the highest local update when present. |
| `{REGION}` | Yes | Yes | Region from title data, when available. |
| `{TYPE}` | No | Yes | File content type: `BASE`, `UPD`, or `DLC`. |
| `{DLC_NAME}` | No | Yes | DLC name from title data. It is empty when no DLC name applies. |
| `{SIZE_GB}` | No | Yes | Physical file size in decimal GB, to one decimal place, rounded to nearest with ties up. Includes the `GB` suffix, for example `0.6GB`. |
| `{SIZE_MB}` | No | Yes | Physical file size in decimal MB, rounded to the nearest whole MB with ties up. Includes the `MB` suffix, for example `600MB`. |

Size uses the scanned physical file size: 1 GB is 1,000,000,000 bytes and 1 MB is 1,000,000 bytes. For a file containing multiple logical records, each record refers to the same physical file and therefore has the same size.

## Examples

Folder template:

```text
{TITLE_NAME} [{TITLE_ID}]
```

Example folder name: `Example Adventure [0100E95004039000]`.

File template:

```text
{TITLE_NAME}[{TITLE_ID}][v{VERSION}] {SIZE_GB}
```

With a 600,000,000-byte base file, the resulting name is `Example Adventure[0100E95004039000][v0] 0.6GB.nsp`. Changing the token to `{SIZE_MB}` produces `Example Adventure[0100E95004039000][v0] 600MB.nsp`. A 2,700,000,000-byte file displays as `2.7GB` or `2700MB`.

## Naming behavior

- A file template must contain `{TITLE_NAME}` or `{TITLE_ID}` when file renaming is enabled. A folder template has the same requirement when per-game folders are enabled.
- File extensions are kept from the original file and appended after the template output; do not include an extension in the template.
- Update and DLC files can be placed in the configured `organization.updates_folder` and `organization.dlc_folder`. These subfolders can also apply when per-game folders are disabled.
- When `organization.switch_safe_file_names` is enabled, the generated name is converted to a Switch-safe name.
- Split archives are moved as a set and keep their original part filenames.

See the [settings reference](settings.md#organization-templates) for the related settings and validation requirements.
