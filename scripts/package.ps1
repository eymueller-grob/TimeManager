$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$version = (Get-Content (Join-Path $root 'wails.json') -Raw | ConvertFrom-Json).info.productVersion
if (-not $version) { throw 'wails.json must define info.productVersion' }

wails build
if ($LASTEXITCODE -ne 0) { throw "wails build failed with exit code $LASTEXITCODE" }

$exe = Join-Path $root 'build\bin\TimeManager.exe'
$zip = Join-Path $root "build\bin\TimeManager_$version.zip"
if (Test-Path $zip) { Remove-Item $zip }
Compress-Archive -Path $exe -DestinationPath $zip
Write-Host "Packaged $zip"
