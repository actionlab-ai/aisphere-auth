param(
  [string]$HostName = "127.0.0.1",
  [int]$Port = 3306,
  [string]$Database = "casdoor",
  [string]$User = "root",
  [string]$Password = $env:CASDOOR_MYSQL_PASSWORD,
  [string]$SqlFile = "deployments/casdoor/sql/aisphere-auth-casdoor.sql",
  [string]$MysqlBin = "mysql",
  [string]$MysqlDumpBin = "mysqldump",
  [switch]$UseDocker,
  [string]$DockerImage = "mysql:8.0",
  [switch]$BackupBefore,
  [string]$BackupDir = "backups/casdoor",
  [switch]$CreateDatabase,
  [switch]$AllowDestructive,
  [switch]$PrepareDump,
  [ValidateSet("data-only", "full")]
  [string]$PrepareMode = "data-only",
  [string]$PreparedSql = "",
  [string]$PrepareKeywords = "aisphere,skillhub",
  [switch]$PrepareIncludeUsers,
  [switch]$PrepareOnly,
  [switch]$DryRun,
  [switch]$Yes
)

$ErrorActionPreference = "Stop"

function Show-Usage {
  @"
Casdoor SQL 自动导入工具（PowerShell）

用途：
  将 Casdoor SQL 初始化文件导入到现有 Casdoor MySQL 数据库，避免手工在 Casdoor UI 中点模型、权限、角色、策略。

推荐：
  对完整 mysqldump 使用 -PrepareDump -PrepareMode data-only，脚本会提取 aisphere/skillhub 相关数据并跳过 token/session/record 等运行态表。

示例：
  powershell -ExecutionPolicy Bypass -File .\scripts\casdoor\import-casdoor-sql.ps1 `
    -HostName 127.0.0.1 `
    -Port 3306 `
    -Database casdoor `
    -User root `
    -Password 'your-password' `
    -SqlFile .\casdoor.sql `
    -PrepareDump `
    -PrepareMode data-only `
    -BackupBefore `
    -Yes
"@
}

function Assert-Identifier([string]$Name, [string]$Value, [string]$Pattern) {
  if ($Value -notmatch $Pattern) {
    throw "invalid ${Name}: ${Value}"
  }
}

Assert-Identifier "database" $Database '^[A-Za-z0-9_]+$'
Assert-Identifier "user" $User '^[A-Za-z0-9_-]+$'
Assert-Identifier "port" ([string]$Port) '^[0-9]+$'

if (-not (Test-Path $SqlFile)) {
  throw "SQL file not found: $SqlFile. Put your exported Casdoor SQL at this path or pass -SqlFile."
}

$TempFiles = @()
function New-ClientCnf {
  $file = New-TemporaryFile
  $TempFiles += $file.FullName
  @"
[client]
user=$User
password=$Password
host=$HostName
port=$Port
protocol=tcp
"@ | Set-Content -Encoding ascii $file.FullName
  return $file.FullName
}

function Cleanup-TempFiles {
  foreach ($file in $TempFiles) {
    if ($file -and (Test-Path $file)) {
      Remove-Item -Force $file -ErrorAction SilentlyContinue
    }
  }
}
trap { Cleanup-TempFiles; throw }

if ($PrepareDump) {
  $prepareTool = "scripts/casdoor/prepare-casdoor-sql.py"
  if (-not (Test-Path $prepareTool)) {
    throw "prepare tool not found: $prepareTool"
  }
  if (-not (Get-Command python -ErrorAction SilentlyContinue) -and -not (Get-Command python3 -ErrorAction SilentlyContinue)) {
    throw "python or python3 is required for -PrepareDump"
  }
  $python = if (Get-Command python3 -ErrorAction SilentlyContinue) { "python3" } else { "python" }
  if ([string]::IsNullOrWhiteSpace($PreparedSql)) {
    $PreparedSql = Join-Path ([System.IO.Path]::GetTempPath()) ("aisphere-casdoor-prepared-{0}.sql" -f ([System.Guid]::NewGuid().ToString("N")))
    $TempFiles += $PreparedSql
  }
  Write-Host "[INFO] preparing Casdoor SQL: mode=$PrepareMode output=$PreparedSql"
  $args = @($prepareTool, "--input", $SqlFile, "--output", $PreparedSql, "--mode", $PrepareMode, "--keywords", $PrepareKeywords)
  if ($PrepareIncludeUsers) { $args += "--include-users" }
  & $python @args
  $SqlFile = $PreparedSql
  if ($PrepareOnly) {
    Write-Host "[OK] prepare-only completed: $SqlFile"
    Cleanup-TempFiles
    exit 0
  }
}

if (-not $AllowDestructive) {
  $sample = Get-Content -Raw $SqlFile
  if ($sample -match '(?i)(^|\s)(DROP\s+TABLE|CREATE\s+TABLE|DROP\s+DATABASE)') {
    throw "SQL contains destructive schema statements. Use -PrepareDump -PrepareMode data-only, or pass -AllowDestructive deliberately."
  }
}

function Test-CommandExists([string]$Command) {
  $null -ne (Get-Command $Command -ErrorAction SilentlyContinue)
}

if (-not $UseDocker -and -not (Test-CommandExists $MysqlBin)) {
  if (Test-CommandExists "docker") {
    Write-Warning "mysql client not found, fallback to docker image: $DockerImage"
    $UseDocker = $true
  } else {
    throw "mysql client not found and docker is unavailable. Install mysql client or pass -UseDocker."
  }
}

$ClientCnf = New-ClientCnf

function Invoke-MysqlExec([string]$Sql) {
  if ($DryRun) { return }
  if ($UseDocker) {
    $mount = "${ClientCnf}:/tmp/client.cnf:ro"
    $Sql | docker run --rm -i -v $mount $DockerImage mysql --defaults-extra-file=/tmp/client.cnf
  } else {
    & $MysqlBin --defaults-extra-file=$ClientCnf -e $Sql
  }
}

function Invoke-MysqlFile([string]$Db, [string]$File) {
  if ($DryRun) { return }
  if ($UseDocker) {
    $mount = "${ClientCnf}:/tmp/client.cnf:ro"
    Get-Content -Raw $File | docker run --rm -i -v $mount $DockerImage mysql --defaults-extra-file=/tmp/client.cnf $Db
  } else {
    Get-Content -Raw $File | & $MysqlBin --defaults-extra-file=$ClientCnf $Db
  }
}

function Backup-Database {
  New-Item -ItemType Directory -Force -Path $BackupDir | Out-Null
  $ts = Get-Date -Format "yyyyMMdd_HHmmss"
  $backupFile = Join-Path $BackupDir "${Database}_${ts}.sql"
  Write-Host "[INFO] backup database '$Database' to $backupFile"
  if ($DryRun) { return }

  if ($UseDocker) {
    $mount = "${ClientCnf}:/tmp/client.cnf:ro"
    docker run --rm -i -v $mount $DockerImage mysqldump --defaults-extra-file=/tmp/client.cnf $Database | Set-Content -Encoding utf8 $backupFile
  } else {
    & $MysqlDumpBin --defaults-extra-file=$ClientCnf $Database | Set-Content -Encoding utf8 $backupFile
  }
}

Write-Host "[INFO] Casdoor SQL import plan"
Write-Host "  host          : $HostName"
Write-Host "  port          : $Port"
Write-Host "  database      : $Database"
Write-Host "  user          : $User"
Write-Host "  sql           : $SqlFile"
Write-Host "  useDocker     : $UseDocker"
Write-Host "  backup        : $BackupBefore"
Write-Host "  createDb      : $CreateDatabase"
Write-Host "  destructive   : $AllowDestructive"
Write-Host "  prepareDump   : $PrepareDump"
Write-Host "  prepareMode   : $PrepareMode"
Write-Host "  dryRun        : $DryRun"

if (-not $Yes) {
  $answer = Read-Host "Continue importing SQL into '$Database'? [y/N]"
  if ($answer -notin @("y", "Y", "yes", "YES")) {
    Write-Host "[INFO] cancelled"
    Cleanup-TempFiles
    exit 0
  }
}

if ($CreateDatabase) {
  Write-Host "[INFO] create database if not exists: $Database"
  Invoke-MysqlExec "CREATE DATABASE IF NOT EXISTS ``$Database`` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
}

if ($BackupBefore) {
  Backup-Database
}

Write-Host "[INFO] importing SQL..."
Invoke-MysqlFile $Database $SqlFile
Write-Host "[OK] Casdoor SQL import completed."
Cleanup-TempFiles
