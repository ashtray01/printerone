$ErrorActionPreference = "Stop"

$xpRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$buildDir = Join-Path $xpRoot "build"
$cacheDir = Join-Path $xpRoot ".cache\go-build"
$tempDir = Join-Path $xpRoot ".tmp"

New-Item -ItemType Directory -Force $buildDir, $cacheDir, $tempDir | Out-Null

$env:GOWORK = "off"
$env:GOCACHE = $cacheDir
$env:GOTMPDIR = $tempDir
$env:GOOS = "windows"
$env:GOARCH = "386"
$env:CGO_ENABLED = "0"

Push-Location $xpRoot
try {
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "Local tests failed with exit code $LASTEXITCODE" }
    go build -trimpath -ldflags "-s -w -H=windowsgui" -o (Join-Path $buildDir "PrinterOne-XP-local-x86.exe") ./cmd/printerone-xp
    if ($LASTEXITCODE -ne 0) { throw "Local build failed with exit code $LASTEXITCODE" }
    Get-FileHash (Join-Path $buildDir "PrinterOne-XP-local-x86.exe") -Algorithm SHA256
} finally {
    Pop-Location
}
