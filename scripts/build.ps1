param(
    [string]$Version = "dev",
    [string]$Go = "go"
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$OutputDir = Join-Path $ProjectRoot "bin"
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

$PreviousGOOS = $env:GOOS
$PreviousGOARCH = $env:GOARCH
$PreviousGOMIPS = $env:GOMIPS
$PreviousCGO = $env:CGO_ENABLED
try {
    $env:GOOS = "linux"
    $env:GOARCH = "mipsle"
    $env:GOMIPS = "softfloat"
    $env:CGO_ENABLED = "0"
    & $Go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$Version" -o (Join-Path $OutputDir "mgl03-homekit-bridge") ./cmd/mgl03-homekit-bridge
    if ($LASTEXITCODE -ne 0) {
        throw "Go build failed with exit code $LASTEXITCODE"
    }
}
finally {
    $env:GOOS = $PreviousGOOS
    $env:GOARCH = $PreviousGOARCH
    $env:GOMIPS = $PreviousGOMIPS
    $env:CGO_ENABLED = $PreviousCGO
}

Get-Item (Join-Path $OutputDir "mgl03-homekit-bridge")
