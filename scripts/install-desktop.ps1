$ErrorActionPreference = "Stop"
$bin = Join-Path $env:USERPROFILE ".arthneura\bin"
New-Item -ItemType Directory -Force -Path $bin | Out-Null
$cfgDir = Join-Path $env:APPDATA "Claude"
New-Item -ItemType Directory -Force -Path $cfgDir | Out-Null
$cfg = Join-Path $cfgDir "claude_desktop_config.json"
$data = @{}
if (Test-Path $cfg) {
  $raw = Get-Content $cfg -Raw
  if ($raw.Trim()) { $data = $raw | ConvertFrom-Json }
}
if (-not $data.mcpServers) { $data | Add-Member -NotePropertyName mcpServers -NotePropertyValue (@{}) -Force }
$data.mcpServers.arthneura = @{
  command = (Join-Path $bin "arthneura-mcp.exe")
  args = @()
  env = @{
    MARKET_URL = "https://api.arthneura.com"
    DOOR = "https://id.arthneura.com"
    REGISTER_BIN = (Join-Path $bin "register-me.exe")
    KEYSTORE_PASS = "dev-passphrase"
  }
}
$data | ConvertTo-Json -Depth 6 | Set-Content -Path $cfg -Encoding utf8
Write-Output "WROTE $cfg"
Write-Output "Put arthneura-mcp.exe and register-me.exe in $bin"

# --- Start continuous decide-loop (Windows) ---
$LogDir = "$env:USERPROFILE\.arthneura\logs"
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null

$Action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-NoProfile -WindowStyle Hidden -Command `"cd `$env:USERPROFILE\arthneura-market; .\scripts\run-decide-loop.sh`""
$Trigger = New-ScheduledTaskTrigger -AtLogOn
$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable
$Principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive

Register-ScheduledTask -TaskName "ArthNeuraDecideLoop" -Action $Action -Trigger $Trigger -Settings $Settings -Principal $Principal -Force
Write-Host "Registered continuous decide-loop task (Windows)"
