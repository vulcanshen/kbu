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

# Asks before removing a directory, only when it is there.
function Ask-Remove($dir, $what) {
    if (Test-Path $dir) {
        $answer = Read-Host "Remove $what in ${dir}? [y/N]"
        if ($answer -eq "y" -or $answer -eq "Y") {
            Remove-Item $dir -Recurse -Force
            Write-Host "removed $dir" -ForegroundColor Green
        } else {
            Write-Host "kept $dir" -ForegroundColor Yellow
        }
    }
}

# Where tdp E3 puts the settings (KBU__CONFIG, else $XDG_CONFIG_HOME\kbu,
# else %USERPROFILE%\.config\kbu), the state (in the settings unless
# KBU__STATE moved it) and the data (KBU__DATA, else %USERPROFILE%\.kbu).
if ($env:KBU__CONFIG) {
    $configDir = $env:KBU__CONFIG.Trim()
} elseif ($env:XDG_CONFIG_HOME) {
    $configDir = Join-Path $env:XDG_CONFIG_HOME "kbu"
} else {
    $configDir = Join-Path $env:USERPROFILE ".config\kbu"
}
Ask-Remove $configDir "kbu settings"
if ($env:KBU__STATE -and $env:KBU__STATE.Trim() -ne $configDir) {
    Ask-Remove $env:KBU__STATE.Trim() "kbu state"
}
if ($env:KBU__DATA) {
    Ask-Remove $env:KBU__DATA.Trim() "kbu data"
} else {
    Ask-Remove (Join-Path $env:USERPROFILE ".kbu") "kbu data"
}
# Where an older kbu kept its settings.
Ask-Remove (Join-Path $env:APPDATA "kbu") "kbu settings from an older kbu"

Write-Host ""
Write-Host "kbu uninstalled. Restart your terminal for PATH changes to take effect." -ForegroundColor Green
