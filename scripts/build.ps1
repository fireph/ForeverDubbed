# Compatibility entry point; all build and packaging logic lives in Go.
$ErrorActionPreference = 'Stop'
Push-Location (Join-Path $PSScriptRoot '..')
try {
    & go run ./tools/build @args
    if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
} finally { Pop-Location }
