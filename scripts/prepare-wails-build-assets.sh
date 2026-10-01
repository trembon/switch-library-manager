#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/.." && pwd)"
build_dir="$repo_root/src/build"

mkdir -p "$build_dir/windows"
cp "$repo_root/src/assets/icons/icon.png" "$build_dir/appicon.png"
cp "$repo_root/src/assets/icons/icon.ico" "$build_dir/windows/icon.ico"

echo "Prepared Wails build assets in $build_dir"
