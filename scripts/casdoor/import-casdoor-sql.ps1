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
  [switch]$DryRun,
  [switch]$Yes
)

$ErrorActionPreference = "Stop"

function Show-Usage {
  @"
Casdoor SQL 自动导入工具（PowerShell）

用途：
  将已经准备好的 Casdoor SQL 初始化文件导入到现有 Casdoor MySQL 数据库，避免手工在 Casdoor UI 中点模型、权限、角色、策略。

示例：
  powershell -ExecutionPolicy Bypass -File .\scripts\casdoor\import-casdoor-sql.ps1 `
    -HostName 127.0.0.1 `
    -Port 3306 `
    -Database casdoor `
    -User root `
    -Password 'your-password' `
    -SqlFile .\deployments\casdoor\sql\aisphere-auth-casdoor.sql `
    -BackupBefore `
    -Yes

参数：
  -HostName       Casdoor MySQL 地址，默认 127.0.0.1
  -Port           Casdoor MySQL 端口，默认 3306
  -Database       Casdoor 数据库名，默认 casdoor
  -User           MySQL 用户，默认 root
  -Password       MySQL 密码，也可用环境变量 CASDOOR_MYSQL_PASSWORD
  -SqlFile        要导入的 SQL 文件
  -MysqlBin       mysql 命令路径，默认 mysql
  -UseDocker      不依赖本机 mysql 客户端，改用 docker run mysql:8.0 执行导入
  -DockerImage    -UseDocker 时使用的镜像，默认 mysql:8.0
  -BackupBefore   导入前先 mysqldump 备份目标数据库
  -BackupDir      备份目录，默认 backups/casdoor
  -CreateDatabase 导入前执行 CREATE DATABASE IF NOT EXISTS
  -DryRun         只打印动作，不导入
  -Yes            跳过确认
"@
}

if (-not (Test-Path $SqlFile)) {
  Write-Error "SQL file not found: $SqlFile. Put your exported Casdoor SQL at this path or pass -SqlFile."
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

function Invoke-MysqlExec([string]$Sql) {
  if ($DryRun) { return }
  if ($UseDocker) {
    if ([string]::IsNullOrEmpty($Password)) {
      docker run --rm -i $DockerImage mysql --protocol=tcp -h $HostName -P $Port -u $User -e $Sql
    } else {
      docker run --rm -i $DockerImage mysql --protocol=tcp -h $HostName -P $Port -u $User "-p$Password" -e $Sql
    }
  } else {
    $old = $env:MYSQL_PWD
    $env:MYSQL_PWD = $Password
    try {
      & $MysqlBin --protocol=tcp -h $HostName -P $Port -u $User -e $Sql
    } finally {
      $env:MYSQL_PWD = $old
    }
  }
}

function Invoke-MysqlFile([string]$Db, [string]$File) {
  if ($DryRun) { return }
  if ($UseDocker) {
    if ([string]::IsNullOrEmpty($Password)) {
      Get-Content -Raw $File | docker run --rm -i $DockerImage mysql --protocol=tcp -h $HostName -P $Port -u $User $Db
    } else {
      Get-Content -Raw $File | docker run --rm -i $DockerImage mysql --protocol=tcp -h $HostName -P $Port -u $User "-p$Password" $Db
    }
  } else {
    $old = $env:MYSQL_PWD
    $env:MYSQL_PWD = $Password
    try {
      Get-Content -Raw $File | & $MysqlBin --protocol=tcp -h $HostName -P $Port -u $User $Db
    } finally {
      $env:MYSQL_PWD = $old
    }
  }
}

function Backup-Database {
  New-Item -ItemType Directory -Force -Path $BackupDir | Out-Null
  $ts = Get-Date -Format "yyyyMMdd_HHmmss"
  $backupFile = Join-Path $BackupDir "${Database}_${ts}.sql"
  Write-Host "[INFO] backup database '$Database' to $backupFile"
  if ($DryRun) { return }

  if ($UseDocker) {
    if ([string]::IsNullOrEmpty($Password)) {
      docker run --rm -i $DockerImage mysqldump --protocol=tcp -h $HostName -P $Port -u $User $Database | Set-Content -Encoding utf8 $backupFile
    } else {
      docker run --rm -i $DockerImage mysqldump --protocol=tcp -h $HostName -P $Port -u $User "-p$Password" $Database | Set-Content -Encoding utf8 $backupFile
    }
  } else {
    $old = $env:MYSQL_PWD
    $env:MYSQL_PWD = $Password
    try {
      & $MysqlDumpBin --protocol=tcp -h $HostName -P $Port -u $User $Database | Set-Content -Encoding utf8 $backupFile
    } finally {
      $env:MYSQL_PWD = $old
    }
  }
}

Write-Host "[INFO] Casdoor SQL import plan"
Write-Host "  host      : $HostName"
Write-Host "  port      : $Port"
Write-Host "  database  : $Database"
Write-Host "  user      : $User"
Write-Host "  sql       : $SqlFile"
Write-Host "  useDocker : $UseDocker"
Write-Host "  backup    : $BackupBefore"
Write-Host "  dryRun    : $DryRun"

if (-not $Yes) {
  $answer = Read-Host "Continue importing SQL into '$Database'? [y/N]"
  if ($answer -notin @("y", "Y", "yes", "YES")) {
    Write-Host "[INFO] cancelled"
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
