# kbu uninstaller for Windows
# Usage: irm https://raw.githubusercontent.com/vulcanshen/kbu/main/uninstall.ps1 | iex

$ErrorActionPreference = "Stop"

$installDir = "$env:LOCALAPPDATA\kbu"

if (-not (Test-Path "$installDir\kbu.exe")) {
    Write-Host "kbu not found in $installDir" -ForegroundColor Red
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

# Remove the settings: where tdp E3 puts them (KBU__CONFIG, else
# $XDG_CONFIG_HOME\kbu, else %USERPROFILE%\.config\kbu), and where an older
# kbu kept them (%APPDATA%\kbu).
$configDirs = @()
if ($env:KBU__CONFIG) {
    $configDirs += $env:KBU__CONFIG.Trim()
} elseif ($env:XDG_CONFIG_HOME) {
    $configDirs += Join-Path $env:XDG_CONFIG_HOME "kbu"
} else {
    $configDirs += Join-Path $env:USERPROFILE ".config\kbu"
}
$configDirs += Join-Path $env:APPDATA "kbu"
foreach ($configDir in $configDirs) {
    if (Test-Path $configDir) {
        $answer = Read-Host "Remove kbu settings in $configDir? [y/N]"
        if ($answer -eq "y" -or $answer -eq "Y") {
            Remove-Item $configDir -Recurse -Force
            Write-Host "removed $configDir" -ForegroundColor Green
        } else {
            Write-Host "kept $configDir" -ForegroundColor Yellow
        }
    }
}

Write-Host ""
Write-Host "kbu uninstalled. Restart your terminal for PATH changes to take effect." -ForegroundColor Green
