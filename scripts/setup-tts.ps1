param([switch]$SkipModels)
$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$runtime = Join-Path $root '.runtime'
New-Item -ItemType Directory -Force $runtime | Out-Null
$env:UV_CACHE_DIR = Join-Path $runtime 'uv-cache'
$env:UV_PYTHON_INSTALL_DIR = Join-Path $runtime 'python'
$env:HF_HOME = Join-Path $runtime 'pocket/huggingface'
$env:HF_HUB_DISABLE_SYMLINKS_WARNING = '1'
$env:PYTHONUTF8 = '1'
$env:PYTHONIOENCODING = 'utf-8'
$uv = (Get-Command uv -ErrorAction Stop).Source
& $uv python install 3.12
if ($LASTEXITCODE -ne 0) { throw 'Python installation failed' }
$venv = Join-Path $runtime 'pocket-env'
if (-not (Test-Path (Join-Path $venv 'Scripts/python.exe'))) {
    & $uv venv --python 3.12 --managed-python $venv
    if ($LASTEXITCODE -ne 0) { throw 'Environment creation failed' }
}
$python = Join-Path $venv 'Scripts/python.exe'
& $uv pip install --python $python torch==2.9.1 --index-url https://download.pytorch.org/whl/cpu
if ($LASTEXITCODE -ne 0) { throw 'PyTorch installation failed' }
& $uv pip install --python $python -r (Join-Path $root 'tts/requirements.txt')
if ($LASTEXITCODE -ne 0) { throw 'Speech dependency installation failed' }
& $python (Join-Path $root 'tts/check_runtime.py')
if ($LASTEXITCODE -ne 0) { throw 'Runtime check failed' }
if (-not $SkipModels) {
    & $python (Join-Path $root 'tts/prepare.py')
    if ($LASTEXITCODE -ne 0) { throw 'Model/voice preparation failed' }
}
