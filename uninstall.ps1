# km8 uninstaller for Windows
# Usage: irm https://raw.githubusercontent.com/vulcanshen/km8/main/uninstall.ps1 | iex

$ErrorActionPreference = "Stop"

$installDir = "$env:LOCALAPPDATA\km8"

if (-not (Test-Path "$installDir\km8.exe")) {
    Write-Host "km8 not found in $installDir" -ForegroundColor Red
    exit 1
}

Remove-Item $installDir -Recurse -Force
Write-Host "removed $installDir" -ForegroundColor Green

# Remove from PATH
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -like "*$installDir*") {
    $newPath = ($userPath -split ";" | Where-Object { $_ -ne $installDir }) -join ";"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "removed $installDir from PATH" -ForegroundColor Green
}

# Remove config
$configDir = Join-Path $env:APPDATA "km8"
if (Test-Path $configDir) {
    $answer = Read-Host "Remove config in $configDir? [y/N]"
    if ($answer -eq "y" -or $answer -eq "Y") {
        Remove-Item $configDir -Recurse -Force
        Write-Host "removed $configDir" -ForegroundColor Green
    } else {
        Write-Host "kept $configDir" -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "km8 uninstalled. Restart your terminal for PATH changes to take effect." -ForegroundColor Green
