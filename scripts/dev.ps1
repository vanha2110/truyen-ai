$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
Set-Location -LiteralPath $projectRoot
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    $globalGoBin = Join-Path $env:ProgramFiles 'Go\bin'
    if (-not (Test-Path -LiteralPath (Join-Path $globalGoBin 'go.exe'))) { throw 'Install Go globally first' }
    $env:PATH = $globalGoBin + ';' + $env:PATH
}
$env:GOTOOLCHAIN = 'local'
$env:GOPATH = Join-Path $projectRoot '.cache\gopath'
$env:GOCACHE = Join-Path $projectRoot '.cache\go-build'
$env:PATH = (Join-Path $env:GOPATH 'bin') + ';' + $env:PATH
if (-not (Get-Command wails -ErrorAction SilentlyContinue)) { go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1; if ($LASTEXITCODE -ne 0) { throw 'Wails installation failed' } }
wails dev
