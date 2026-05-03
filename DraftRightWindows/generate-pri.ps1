# Generate resources.pri for an unpackaged WinUI 3 app build.
#
# WinUI 3 resolves theme XAML resources through `ms-appx:///` URIs, which the
# resource manager looks up via PRI files. The standalone .NET SDK doesn't ship
# the AppX MSBuild tooling that would normally produce a resources.pri at the
# app root, so without one the framework can't load themeresources.xaml and
# the first XAML render crashes with STATUS_STOWED_EXCEPTION (0xc000027b).
#
# This script copies the WindowsAppSDK's Microsoft.UI.Xaml.Controls.pri (which
# already ships beside the published exe) over to resources.pri, giving the
# resource manager an entry point that chains to the framework PRIs. That's
# enough to satisfy lookups for the theme dictionaries pulled in by ProgressRing,
# Button, etc.
#
# Run this AFTER `dotnet publish`, against the publish folder. Required only on
# unpackaged builds; CI builds with EnableMsixTooling=true generate a real
# resources.pri and don't need this.

param(
    [string]$AppDir = "$PSScriptRoot\publish"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $AppDir)) {
    Write-Error "App directory not found: $AppDir"
    exit 1
}

$src = Join-Path $AppDir "Microsoft.UI.Xaml.Controls.pri"
$dst = Join-Path $AppDir "resources.pri"

if (-not (Test-Path $src)) {
    Write-Error "Microsoft.UI.Xaml.Controls.pri not found in $AppDir — was this built self-contained?"
    exit 1
}

Copy-Item $src $dst -Force
$size = (Get-Item $dst).Length
Write-Host "Wrote $dst ($([math]::Round($size/1MB,2)) MB)"
