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
  [string]$PrepareKeywords = "aisphere,aihub",
  [switch]$PrepareIncludeUsers,
  [switch]$PrepareOnly,
  [switch]$Seed,
  [switch]$SeedOnly,
  [string]$SeedOutput = "",
  [string]$SeedEnvOutput = "",
  [string]$SeedOrg = "aisphere",
  [string]$SeedOrgDisplayName = "AI Sphere",
  [string]$SeedAppOwner = "admin",
  [string]$SeedApp = "aisphere-auth",
  [string]$SeedAppDisplayName = "AI Sphere Auth",
  [string]$SeedClientId = "aisphere-auth",
  [string]$SeedClientSecret = "",
  [string[]]$SeedRedirectUri = @(),
  [string]$SeedCert = "cert-built-in",
  [string]$SeedModel = "aisphere-auth-model",
  [string]$SeedPermissionId = "perm_platform_admin",
  [string]$SeedAdminUser = "admin",
  [switch]$SeedSkipAdminBinding,
  [switch]$DryRun,
  [switch]$Yes
)

$ErrorActionPreference = "Stop"

function Show-Usage {
  @"
Casdoor SQL 自动导入工具（PowerShell）

推荐开箱即用：
  powershell -ExecutionPolicy Bypass -File .\scripts\casdoor\import-casdoor-sql.ps1 `
    -Seed `
    -SeedOrg aisphere `
    -SeedApp aisphere-auth `
    -SeedClientId aisphere-auth `
    -SeedClientSecret 'replace-with-oauth-secret' `
    -SeedRedirectUri http://127.0.0.1:18080/auth/callback/casdoor `
    -HostName 127.0.0.1 `
    -Port 3306 `
    -Database casdoor `
    -User root `
    -Password 'your-password' `
    -BackupBefore `
    -Yes

只生成 seed SQL，不连接 MySQL：
  powershell -ExecutionPolicy Bypass -File .\scripts\casdoor\import-casdoor-sql.ps1 `
    -Seed `
    -SeedOnly `
    -SeedOutput .\deployments\casdoor\sql\aisphere-auth-casdoor.sql `
    -SeedEnvOutput .\.env.casdoor.generated `
    -SeedClientSecret 'replace-with-oauth-secret'

迁移旧 Casdoor dump：
  使用 -PrepareDump -PrepareMode data-only。
"@
}

function Test-AisphereIdentifier {
  param(
    [Parameter(Mandatory = $true)][string]$Name,
    [Parameter(Mandatory = $true)][string]$Value,
    [Parameter(Mandatory = $true)][string]$Pattern
  )
  if ($Value -notmatch $Pattern) {
    throw "invalid ${Name}: ${Value}"
  }
}

function Get-PythonCommand {
  if (Get-Command python3 -ErrorAction SilentlyContinue) { return "python3" }
  if (Get-Command python -ErrorAction SilentlyContinue) { return "python" }
  throw "python or python3 is required"
}

$TempFiles = @()
function Cleanup-TempFiles {
  foreach ($file in $TempFiles) {
    if ($file -and (Test-Path $file)) {
      Remove-Item -Force $file -ErrorAction SilentlyContinue
    }
  }
}
trap { Cleanup-TempFiles; throw }

function New-AisphereMysqlClientCnf {
  $file = New-TemporaryFile
  $script:TempFiles += $file.FullName
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

function Test-CommandExists([string]$Command) {
  $null -ne (Get-Command $Command -ErrorAction SilentlyContinue)
}

function Render-SeedSql {
  $renderer = "scripts/casdoor/render-casdoor-seed.py"
  if (-not (Test-Path $renderer)) { throw "seed renderer not found: $renderer" }
  $python = Get-PythonCommand
  if ([string]::IsNullOrWhiteSpace($SeedOutput)) {
    $script:SeedOutput = Join-Path ([System.IO.Path]::GetTempPath()) ("aisphere-casdoor-seed-{0}.sql" -f ([System.Guid]::NewGuid().ToString("N")))
    $script:TempFiles += $script:SeedOutput
  }
  Write-Host "[INFO] rendering project Casdoor seed SQL: $SeedOutput"
  $args = @($renderer, "--output", $SeedOutput, "--org", $SeedOrg, "--org-display-name", $SeedOrgDisplayName, "--app-owner", $SeedAppOwner, "--app", $SeedApp, "--app-display-name", $SeedAppDisplayName, "--client-id", $SeedClientId, "--cert", $SeedCert, "--model", $SeedModel, "--permission-id", $SeedPermissionId, "--admin-user", $SeedAdminUser)
  if (-not [string]::IsNullOrWhiteSpace($SeedClientSecret)) { $args += @("--client-secret", $SeedClientSecret) }
  if (-not [string]::IsNullOrWhiteSpace($SeedEnvOutput)) { $args += @("--env-output", $SeedEnvOutput) }
  if ($SeedSkipAdminBinding) { $args += "--skip-admin-binding" }
  if ($SeedRedirectUri.Count -eq 0) {
    $args += @("--redirect-uri", "http://127.0.0.1:18080/auth/callback/casdoor")
  } else {
    foreach ($uri in $SeedRedirectUri) { $args += @("--redirect-uri", $uri) }
  }
  & $python @args
  if ($LASTEXITCODE -ne 0) { throw "seed renderer failed with exit code $LASTEXITCODE" }
  $script:SqlFile = $SeedOutput
  if ($SeedOnly) {
    Write-Host "[OK] seed-only completed: $SqlFile"
    Cleanup-TempFiles
    exit 0
  }
}

function Prepare-DumpSql {
  $prepareTool = "scripts/casdoor/prepare-casdoor-sql.py"
  if (-not (Test-Path $prepareTool)) { throw "prepare tool not found: $prepareTool" }
  $python = Get-PythonCommand
  if ([string]::IsNullOrWhiteSpace($PreparedSql)) {
    $script:PreparedSql = Join-Path ([System.IO.Path]::GetTempPath()) ("aisphere-casdoor-prepared-{0}.sql" -f ([System.Guid]::NewGuid().ToString("N")))
    $script:TempFiles += $script:PreparedSql
  }
  Write-Host "[INFO] preparing Casdoor SQL: mode=$PrepareMode output=$PreparedSql"
  $args = @($prepareTool, "--input", $SqlFile, "--output", $PreparedSql, "--mode", $PrepareMode, "--keywords", $PrepareKeywords)
  if ($PrepareIncludeUsers) { $args += "--include-users" }
  & $python @args
  if ($LASTEXITCODE -ne 0) { throw "prepare tool failed with exit code $LASTEXITCODE" }
  $script:SqlFile = $PreparedSql
  if ($PrepareOnly) {
    Write-Host "[OK] prepare-only completed: $SqlFile"
    Cleanup-TempFiles
    exit 0
  }
}

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

Test-AisphereIdentifier -Name "database" -Value $Database -Pattern '^[A-Za-z0-9_]+$'
Test-AisphereIdentifier -Name "user" -Value $User -Pattern '^[A-Za-z0-9_-]+$'
Test-AisphereIdentifier -Name "port" -Value ([string]$Port) -Pattern '^[0-9]+$'
if ($Seed -and $PrepareDump) { throw "-Seed and -PrepareDump are mutually exclusive" }

if ($Seed) {
  Render-SeedSql
} elseif (-not (Test-Path $SqlFile)) {
  throw "SQL file not found: $SqlFile. Use -Seed to render project seed SQL or pass -SqlFile."
}

if ($PrepareDump) { Prepare-DumpSql }

if (-not $AllowDestructive) {
  $sample = Get-Content -Raw $SqlFile
  if ($sample -match '(?i)(^|\s)(DROP\s+TABLE|CREATE\s+TABLE|DROP\s+DATABASE)') {
    throw "SQL contains destructive schema statements. Use -Seed for project bootstrap, -PrepareDump -PrepareMode data-only for migration, or pass -AllowDestructive deliberately."
  }
}

if (-not $UseDocker -and -not (Test-CommandExists $MysqlBin)) {
  if (Test-CommandExists "docker") {
    Write-Warning "mysql client not found, fallback to docker image: $DockerImage"
    $UseDocker = $true
  } else {
    throw "mysql client not found and docker is unavailable. Install mysql client or pass -UseDocker."
  }
}

$ClientCnf = New-AisphereMysqlClientCnf

Write-Host "[INFO] Casdoor SQL import plan"
Write-Host "  host          : $HostName"
Write-Host "  port          : $Port"
Write-Host "  database      : $Database"
Write-Host "  user          : $User"
Write-Host "  sql           : $SqlFile"
Write-Host "  seed          : $Seed"
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

if ($BackupBefore) { Backup-Database }

Write-Host "[INFO] importing SQL..."
Invoke-MysqlFile $Database $SqlFile
Write-Host "[OK] Casdoor SQL import completed."
Cleanup-TempFiles
