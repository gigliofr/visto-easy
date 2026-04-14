param(
  [string]$Port = "8080",
  [string]$MongoUri = "",
  [string]$MongoDbName = "visto-easy",
  [string]$JwtSecret = "",
  [string]$BackofficeSeedPassword = "",
  [switch]$KeepExistingServer
)

$ErrorActionPreference = "Stop"

function Ensure-EnvValue {
  param(
    [string]$Name,
    [string]$Preferred,
    [string]$Fallback
  )

  if (-not [string]::IsNullOrWhiteSpace($Preferred)) {
    Set-Item -Path ("Env:" + $Name) -Value $Preferred
    return
  }

  $current = (Get-Item -Path ("Env:" + $Name) -ErrorAction SilentlyContinue).Value
  if (-not [string]::IsNullOrWhiteSpace($current)) {
    return
  }

  if (-not [string]::IsNullOrWhiteSpace($Fallback)) {
    Set-Item -Path ("Env:" + $Name) -Value $Fallback
  }
}

Ensure-EnvValue -Name "PORT" -Preferred $Port -Fallback "8080"
Ensure-EnvValue -Name "MONGODB_DB_NAME" -Preferred $MongoDbName -Fallback "visto-easy"
Ensure-EnvValue -Name "JWT_SECRET" -Preferred $JwtSecret -Fallback "vistoeasy-local-dev-secret-key-2026-verylong"
Ensure-EnvValue -Name "BACKOFFICE_SEED_PASSWORD" -Preferred $BackofficeSeedPassword -Fallback "Admin123!Seed2026"

if (-not [string]::IsNullOrWhiteSpace($MongoUri)) {
  $env:MONGODB_URI = $MongoUri
}

if ([string]::IsNullOrWhiteSpace($env:MONGODB_URI)) {
  Write-Error "MONGODB_URI non impostata. Passa -MongoUri oppure imposta la variabile ambiente."
}

$listenPort = [int]$env:PORT
$listeners = Get-NetTCPConnection -LocalPort $listenPort -State Listen -ErrorAction SilentlyContinue

if ($listeners -and -not $KeepExistingServer) {
  $pids = $listeners | Select-Object -ExpandProperty OwningProcess -Unique
  foreach ($procId in $pids) {
    $proc = Get-Process -Id $procId -ErrorAction SilentlyContinue
    if ($null -eq $proc) {
      continue
    }

    if ($proc.ProcessName -ieq "visto-easy") {
      Write-Host "[start_dev] termino processo visto-easy PID=$procId su porta $listenPort"
      Stop-Process -Id $procId -Force
    } else {
      Write-Error "Porta $listenPort occupata da processo '$($proc.ProcessName)' (PID=$procId). Usa -KeepExistingServer o cambia -Port."
    }
  }
}

Write-Host "[start_dev] PORT=$($env:PORT)"
Write-Host "[start_dev] MONGODB_DB_NAME=$($env:MONGODB_DB_NAME)"
Write-Host "[start_dev] avvio go run ."

go run .
