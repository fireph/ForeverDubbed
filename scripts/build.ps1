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
        Copy-Item data (Join-Path $stage 'data') -Recurse
        Copy-Item Start-ForeverDubbed.cmd, Setup-PocketTTS.cmd $stage
        New-Item -ItemType Directory (Join-Path $stage 'tts'), (Join-Path $stage 'scripts') | Out-Null
        Copy-Item tts/server.py, tts/engine.py, tts/runtime.py, tts/prepare.py, tts/check_runtime.py, tts/clone_voice.py, tts/requirements.txt, tts/voices.json (Join-Path $stage 'tts')
        # Include only local voice files used by this configuration, never the
        # whole custom directory (which contains recordings and local backups).
        $ttsRoot = (Resolve-Path tts).Path
        $customRoot = Join-Path $ttsRoot 'custom'
        $voiceConfig = Get-Content tts/voices.json -Raw | ConvertFrom-Json
        $voiceFiles = $voiceConfig.profiles.PSObject.Properties | ForEach-Object { $_.Value.voice } |
            Where-Object { $_ -match '\.(safetensors|wav)$' } | Sort-Object -Unique
        foreach ($voiceFile in $voiceFiles) {
            $source = [System.IO.Path]::GetFullPath((Join-Path $ttsRoot $voiceFile))
            if (-not $source.StartsWith(($customRoot + '\'), [System.StringComparison]::OrdinalIgnoreCase) -or
                $source.EndsWith('.pending.safetensors', [System.StringComparison]::OrdinalIgnoreCase)) {
                throw "Bundle voice references must be finished files under tts/custom: $voiceFile"
            }
            $destination = Join-Path (Join-Path $stage 'tts') $source.Substring($ttsRoot.Length + 1)
            New-Item -ItemType Directory -Force (Split-Path $destination -Parent) | Out-Null
            Copy-Item -LiteralPath $source -Destination $destination
        }
        Copy-Item scripts/start.ps1, scripts/setup-tts.ps1 (Join-Path $stage 'scripts')
        New-Item -ItemType Directory (Join-Path $stage 'addon') | Out-Null
        Copy-Item addon/ForeverDubbed (Join-Path $stage 'addon') -Recurse
        Compress-Archive -Path "$stage/*" -DestinationPath dist/ForeverDubbed-windows-amd64.zip -Force
    } finally { Remove-Item $stage -Recurse -Force }
    Write-Host 'Built dist/foreverdubbed.exe and addon/Windows ZIP packages.'
} finally { Pop-Location }
