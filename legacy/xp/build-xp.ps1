param(
    [string]$GoRoot = ""
)

$ErrorActionPreference = "Stop"
$xpRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
if ($GoRoot -eq "") {
    $GoRoot = Join-Path $xpRoot ".toolchains\go1.10.8\go"
}
$goExe = Join-Path $GoRoot "bin\go.exe"
if (-not (Test-Path -LiteralPath $goExe)) {
    throw "Go 1.10.8 not found at $goExe"
}

$runId = [Guid]::NewGuid().ToString("N")
$gopathRoot = Join-Path $xpRoot ".tmp\go110-$runId"
$sourceRoot = Join-Path $gopathRoot "src\github.com\ashtray01\printerone\legacy\xp"
$buildDir = Join-Path $xpRoot "build"
$resourceFile = Join-Path $xpRoot "cmd\printerone-xp\rsrc_windows_386.syso"
if (-not (Test-Path -LiteralPath $resourceFile)) {
    throw "Required XP icon resource is missing: $resourceFile"
}
New-Item -ItemType Directory -Force $sourceRoot, $buildDir | Out-Null

foreach ($directory in @("cmd", "config", "platform", "printdata", "receiver", "sessionlog", "spooler")) {
    Copy-Item -LiteralPath (Join-Path $xpRoot $directory) -Destination $sourceRoot -Recurse
}

$env:GOROOT = $GoRoot
$env:GOPATH = $gopathRoot
$env:GOOS = "windows"
$env:GOARCH = "386"
$env:CGO_ENABLED = "0"
$env:GOCACHE = "off"

Push-Location $sourceRoot
try {
    & $goExe test ./...
    if ($LASTEXITCODE -ne 0) { throw "Go 1.10.8 tests failed with exit code $LASTEXITCODE" }
    & $goExe build -ldflags "-s -w -H=windowsgui" -o (Join-Path $buildDir "PrinterOne-XP-SP3-x86.exe") ./cmd/printerone-xp
    if ($LASTEXITCODE -ne 0) { throw "Go 1.10.8 build failed with exit code $LASTEXITCODE" }
    $exePath = Join-Path $buildDir "PrinterOne-XP-SP3-x86.exe"
    $hash = (Get-FileHash $exePath -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  PrinterOne-XP-SP3-x86.exe" | Out-File (Join-Path $buildDir "PrinterOne-XP-SP3-x86.exe.sha256") -Encoding ascii -NoNewline
    Get-FileHash $exePath -Algorithm SHA256
} finally {
    Pop-Location
}
