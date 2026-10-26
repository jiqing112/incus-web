$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$version = (Get-Date -Format "yyyyMMdd-HHmmss")
$outputDir = "build"
New-Item -ItemType Directory -Force -Path $outputDir | Out-Null

Write-Host "=== Building incus-web ===" -ForegroundColor Cyan

Write-Host "  [1/5] Frontend (vite build)..." -ForegroundColor Yellow
Push-Location frontend
npm install --no-audit --no-fund
if ($LASTEXITCODE -ne 0) { Pop-Location; Write-Host "  npm install FAILED" -ForegroundColor Red; exit 1 }
npm run build
if ($LASTEXITCODE -ne 0) { Pop-Location; Write-Host "  vite build FAILED" -ForegroundColor Red; exit 1 }
Pop-Location
Write-Host "  OK: frontend/dist" -ForegroundColor Green

Write-Host "  [2/5] Sync dist -> handler/dist..." -ForegroundColor Yellow
# 用 robocopy /MIR 做镜像同步：幂等、不需要先删除目录。
# （Remove-Item -Recurse 在启用了删除保护/回收站策略的环境里会被拦截，导致构建中断）
New-Item -ItemType Directory -Force -Path handler/dist | Out-Null
robocopy frontend/dist handler/dist /MIR /NFL /NDL /NJH /NJS /NP | Out-Null
if ($LASTEXITCODE -ge 8) { Write-Host "  robocopy FAILED (exit $LASTEXITCODE)" -ForegroundColor Red; exit 1 }
Write-Host "  OK" -ForegroundColor Green

Write-Host "  [3/5] Windows AMD64..." -ForegroundColor Yellow
$env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
go build -ldflags "-s -w -X main.Version=$version" -o "$outputDir/incus-web-$version-win-amd64.exe" .
if ($LASTEXITCODE -ne 0) { Write-Host "  FAILED" -ForegroundColor Red; exit 1 }
Write-Host "  OK: $outputDir/incus-web-$version-win-amd64.exe" -ForegroundColor Green

Write-Host "  [4/5] Linux AMD64..." -ForegroundColor Yellow
$env:GOOS = "linux"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
go build -ldflags "-s -w -X main.Version=$version" -o "$outputDir/incus-web-$version-linux-amd64" .
if ($LASTEXITCODE -ne 0) { Write-Host "  FAILED" -ForegroundColor Red; exit 1 }
Write-Host "  OK: $outputDir/incus-web-$version-linux-amd64" -ForegroundColor Green

# 树莓派 / Graviton 等 arm64 宿主机同样需要面板二进制，否则安装向导无从触发
Write-Host "  [5/5] Linux ARM64..." -ForegroundColor Yellow
$env:GOOS = "linux"; $env:GOARCH = "arm64"; $env:CGO_ENABLED = "0"
go build -ldflags "-s -w -X main.Version=$version" -o "$outputDir/incus-web-$version-linux-arm64" .
if ($LASTEXITCODE -ne 0) { Write-Host "  FAILED" -ForegroundColor Red; exit 1 }
Write-Host "  OK: $outputDir/incus-web-$version-linux-arm64" -ForegroundColor Green

Write-Host ""
Write-Host "=== Build complete ===" -ForegroundColor Cyan
Write-Host ""
Get-ChildItem $outputDir | ForEach-Object { 
    $size = "{0:N0}" -f ($_.Length / 1KB)
    Write-Host "  $($_.Name)  (${size} KB)" 
}
