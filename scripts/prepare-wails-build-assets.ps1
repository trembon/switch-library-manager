Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$buildDir = Join-Path $repoRoot 'src\build'

New-Item -ItemType Directory -Force -Path (Join-Path $buildDir 'windows') | Out-Null
Copy-Item -LiteralPath (Join-Path $repoRoot 'src\assets\icons\icon.png') -Destination (Join-Path $buildDir 'appicon.png') -Force
Copy-Item -LiteralPath (Join-Path $repoRoot 'src\assets\icons\icon.ico') -Destination (Join-Path $buildDir 'windows\icon.ico') -Force

Write-Output "Prepared Wails build assets in $buildDir"
