param([switch]$Installer)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
Set-Location -LiteralPath $projectRoot
$globalGo = Get-Command go -ErrorAction SilentlyContinue
if (-not $globalGo) {
    $standardGo = Join-Path $env:ProgramFiles 'Go\bin\go.exe'
    if (-not (Test-Path -LiteralPath $standardGo)) { throw 'Go global is required. Install Go 1.26+ and reopen the terminal.' }
    $env:PATH = (Split-Path $standardGo -Parent) + ';' + $env:PATH
}
$env:GOTOOLCHAIN = 'local'
$env:GOPATH = Join-Path $projectRoot '.cache\gopath'
$env:GOCACHE = Join-Path $projectRoot '.cache\go-build'
$env:PATH = (Join-Path $env:GOPATH 'bin') + ';' + $env:PATH
$localNSIS = Join-Path $projectRoot '.tools\nsis'
if (Test-Path -LiteralPath (Join-Path $localNSIS 'makensis.exe')) { $env:PATH = $localNSIS + ';' + $env:PATH }
if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
    if ($LASTEXITCODE -ne 0) { throw 'Could not install Wails CLI' }
}
go test ./... -count=1
if ($LASTEXITCODE -ne 0) { throw 'Go tests failed' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed' }
$buildArgs = @('build', '-trimpath', '-o', 'ConvertFix-0.1.0.exe')
if ($Installer) { $buildArgs += @('-nsis', '-installscope', 'user') }
wails @buildArgs
if ($LASTEXITCODE -ne 0) { throw 'Desktop build failed' }
go build -trimpath -o build/bin/convertfix-cli.exe ./cmd/cli
if ($LASTEXITCODE -ne 0) { throw 'CLI build failed' }
Get-ChildItem -LiteralPath 'build/bin' -Filter '*.exe' | ForEach-Object { Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256 } | Format-Table
