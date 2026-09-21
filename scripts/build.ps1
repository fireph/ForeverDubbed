$ErrorActionPreference = 'Stop'
Push-Location (Join-Path $PSScriptRoot '..')
try {
    New-Item -ItemType Directory -Force dist | Out-Null
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
    $previousGoos, $previousGoarch = $env:GOOS, $env:GOARCH
    try {
        $env:GOOS = 'windows'
        $env:GOARCH = 'amd64'
        go build -buildvcs=false -trimpath -o dist/foreverdubbed.exe ./cmd/foreverdubbed
        if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
    } finally {
        $env:GOOS = $previousGoos
        $env:GOARCH = $previousGoarch
    }
    Compress-Archive -Path addon/ForeverDubbed -DestinationPath dist/ForeverDubbed-addon.zip -Force
    $stage = Join-Path ([System.IO.Path]::GetTempPath()) ('ForeverDubbed-' + [guid]::NewGuid().ToString())
    New-Item -ItemType Directory $stage | Out-Null
    try {
        Copy-Item dist/foreverdubbed.exe, README.md, PROTOCOL.md, CHANGELOG.md $stage
        Copy-Item docs (Join-Path $stage 'docs') -Recurse
        Copy-Item Start-ForeverDubbed.cmd, Setup-PocketTTS.cmd $stage
        New-Item -ItemType Directory (Join-Path $stage 'tts'), (Join-Path $stage 'scripts') | Out-Null
        Copy-Item tts/server.py, tts/runtime.py, tts/prepare.py, tts/check_runtime.py, tts/requirements.txt, tts/voices.json (Join-Path $stage 'tts')
        Copy-Item scripts/start.ps1, scripts/setup-tts.ps1 (Join-Path $stage 'scripts')
        New-Item -ItemType Directory (Join-Path $stage 'addon') | Out-Null
        Copy-Item addon/ForeverDubbed (Join-Path $stage 'addon') -Recurse
        Compress-Archive -Path "$stage/*" -DestinationPath dist/ForeverDubbed-windows-amd64.zip -Force
    } finally { Remove-Item $stage -Recurse -Force }
    Write-Host 'Built dist/foreverdubbed.exe and addon/Windows ZIP packages.'
} finally { Pop-Location }
