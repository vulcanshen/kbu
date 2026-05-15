# km8 installer for Windows
# Usage: irm https://raw.githubusercontent.com/vulcanshen/km8/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$repo = "vulcanshen/km8"
$installDir = "$env:LOCALAPPDATA\km8"

# Detect architecture
$arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else {
    Write-Host "Error: 32-bit systems are not supported." -ForegroundColor Red
    exit 1
}

# Get latest release tag
Write-Host "Fetching latest release..." -ForegroundColor Cyan
$release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
$version = $release.tag_name -replace "^v", ""
Write-Host "Latest version: $version"

# Download
$fileName = "km8_${version}_windows_${arch}.zip"
$downloadUrl = "https://github.com/$repo/releases/download/$($release.tag_name)/$fileName"
$tempFile = Join-Path $env:TEMP $fileName

Write-Host "Downloading $fileName..." -ForegroundColor Cyan
Invoke-WebRequest -Uri $downloadUrl -OutFile $tempFile -UseBasicParsing

# Extract
if (Test-Path $installDir) {
    try {
        Remove-Item $installDir -Recurse -Force -ErrorAction Stop
    } catch {
        Write-Host ""
        Write-Host "Error: Cannot update km8 — the file is in use." -ForegroundColor Red
        Write-Host "Please close km8 first, then run this installer again." -ForegroundColor Yellow
        Remove-Item $tempFile -ErrorAction SilentlyContinue
        exit 1
    }
}
New-Item -ItemType Directory -Path $installDir -Force | Out-Null
Expand-Archive -Path $tempFile -DestinationPath $installDir -Force
Remove-Item $tempFile

# Add to PATH if not already there
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
    Write-Host "Added $installDir to PATH." -ForegroundColor Green
    Write-Host "Please restart your terminal for PATH changes to take effect." -ForegroundColor Yellow
} else {
    Write-Host "$installDir is already in PATH." -ForegroundColor Green
}

Write-Host ""
Write-Host "km8 $version installed successfully!" -ForegroundColor Green
Write-Host "Run 'km8 --version' to verify." -ForegroundColor Cyan
