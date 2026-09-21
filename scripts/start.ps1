# Start one local Pocket TTS helper, then run the Go screen reader. Stop only the
# helper we started when the reader exits (including Ctrl+C).
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$python = Join-Path $root '.runtime/pocket-env/Scripts/python.exe'
$exe = Join-Path $root 'foreverdubbed.exe'
if (-not (Test-Path $exe)) { $exe = Join-Path $root 'dist/foreverdubbed.exe' }
$config = Join-Path $root 'tts/voices.json'
$endpoint = (Get-Content $config -Raw | ConvertFrom-Json).endpoint.TrimEnd('/')
$uri = [uri]$endpoint
if ($uri.Scheme -ne 'http' -or $uri.Host -notin @('127.0.0.1', 'localhost', '[::1]')) {
    throw 'Voice config endpoint must be a loopback HTTP address'
}
# The supplied launcher uses IPv4 loopback, matching the service binding.
if ($uri.Host -ne '127.0.0.1') { throw 'Use 127.0.0.1 in voices.json with this launcher' }
if (-not (Test-Path $exe)) { throw 'Build the companion first: scripts/build.ps1' }
function Get-Health {
    try { return Invoke-RestMethod "$endpoint/health" -TimeoutSec 2 } catch { return $null }
}
$service = $null
try {
    $health = Get-Health
    if ($null -eq $health) {
        if (-not (Test-Path $python) -or -not (Test-Path (Join-Path $root '.runtime/pocket/ready.json'))) {
            throw 'Run Setup-PocketTTS.cmd once to install the local speech runtime and voices.'
        }
        $env:PYTHONUTF8 = '1'
        $env:PYTHONIOENCODING = 'utf-8'
        $log = Join-Path $root '.runtime/pocket/server.log'
        $errLog = Join-Path $root '.runtime/pocket/server-errors.log'
        # Explicit quoted paths handle spaces; no dialogue is passed as code.
        $arguments = @('-u', ('"' + (Join-Path $root 'tts/server.py') + '"'), '--config', ('"' + $config + '"'), '--port', $uri.Port)
        $service = Start-Process -FilePath $python -ArgumentList $arguments -PassThru -WindowStyle Hidden -RedirectStandardOutput $log -RedirectStandardError $errLog
        Write-Host 'Loading Pocket TTS on CPU...'
        $deadline = [DateTime]::UtcNow.AddSeconds(90)
        while ($null -eq $health -and [DateTime]::UtcNow -lt $deadline) {
            if ($service.HasExited) { throw "Pocket TTS exited. See $errLog" }
            Start-Sleep -Milliseconds 300
            $health = Get-Health
        }
        if ($null -eq $health) { throw "Pocket TTS startup timed out. See $errLog" }
    }
    if ($health.engine -ne 'pocket-tts') { throw "Another application is using $endpoint" }
    $digest = (Get-FileHash $config -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($health.config_digest -ne $digest) { throw 'An existing Pocket TTS helper uses different voices. Close its launcher and start again.' }
    & $exe -voice-config $config @args
    if ($LASTEXITCODE -ne 0) { throw "ForeverDubbed exited with code $LASTEXITCODE" }
} finally {
    if ($null -ne $service -and -not $service.HasExited) {
        # Python venv launchers may have a child process; stop the entire owned tree.
        & taskkill.exe /PID $service.Id /T /F 2>$null | Out-Null
    }
}
