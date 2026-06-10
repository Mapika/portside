# portside installer for Windows:
#   irm https://raw.githubusercontent.com/Mapika/portside/main/install.ps1 | iex
$ErrorActionPreference = "Stop"

$repo = "Mapika/portside"
$arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
$dest = Join-Path $env:LOCALAPPDATA "Programs\portside"

$url = "https://github.com/$repo/releases/latest/download/portside_windows_$arch.zip"
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) "portside_install"
Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $tmp | Out-Null

Write-Host "downloading $url"
Invoke-WebRequest -Uri $url -OutFile (Join-Path $tmp "portside.zip")
Expand-Archive -Path (Join-Path $tmp "portside.zip") -DestinationPath $tmp -Force

New-Item -ItemType Directory -Force $dest | Out-Null
Copy-Item (Join-Path $tmp "portside.exe") $dest -Force
Copy-Item (Join-Path $tmp "scripts\work.ps1") $dest -Force
Copy-Item (Join-Path $tmp "scripts\work.cmd") $dest -Force
Copy-Item (Join-Path $tmp "scripts\work.cmd") (Join-Path $dest "work.bat") -Force
Remove-Item -Recurse -Force $tmp

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (($userPath -split ";") -notcontains $dest) {
    $newPath = if ($userPath) { "$userPath;$dest" } else { $dest }
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "added $dest to your user PATH (restart the terminal to pick it up)"
}
Write-Host "installed portside and work to $dest"
