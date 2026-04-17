param(
    [string]$Url,
    [string]$Key,
    [string]$Platform = "claude"
)

# Strip trailing slashes from URL
$Url = $Url.TrimEnd('/')

if (-not $Url -or -not $Key) {
    Write-Host "Usage: .\setup.ps1 -Url <url> -Key <key> [-Platform claude|codex]"
    exit 1
}

if ($Platform -eq "claude") {
    [System.Environment]::SetEnvironmentVariable("ANTHROPIC_AUTH_TOKEN", $Key, "User")
    [System.Environment]::SetEnvironmentVariable("ANTHROPIC_BASE_URL", $Url, "User")

    # Also set for current session
    $env:ANTHROPIC_AUTH_TOKEN = $Key
    $env:ANTHROPIC_BASE_URL = $Url

    Write-Host "`n✅ Claude Code configured successfully!"
    Write-Host "   ANTHROPIC_AUTH_TOKEN and ANTHROPIC_BASE_URL set as user environment variables."
    Write-Host "   Restart your terminal to apply."
}
elseif ($Platform -eq "codex") {
    $codexDir = "$env:USERPROFILE\.codex"
    if (!(Test-Path $codexDir)) {
        New-Item -ItemType Directory -Path $codexDir | Out-Null
    }

    $configToml = @"
model_provider = "MikuCode"
model = "gpt-5.4"
model_reasoning_effort = "high"
disable_response_storage = true
preferred_auth_method = "apikey"
[model_providers.MikuCode]
name = "MikuCode"
base_url = "$Url/v1"
wire_api = "responses"
requires_openai_auth = true
"@

    $authJson = @"
{
  "OPENAI_API_KEY": "$Key"
}
"@

    # Write UTF-8 without BOM — PowerShell 5.1's `Set-Content -Encoding UTF8`
    # emits a BOM (EF BB BF), which breaks Codex's TOML/JSON parsers.
    $utf8NoBom = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText("$codexDir\config.toml", $configToml, $utf8NoBom)
    [System.IO.File]::WriteAllText("$codexDir\auth.json",   $authJson,   $utf8NoBom)

    Write-Host "`n✅ Codex configured successfully!"
    Write-Host "   Config written to $codexDir\config.toml"
    Write-Host "   Auth written to $codexDir\auth.json"
}
else {
    Write-Host "Unknown platform: $Platform (use 'claude' or 'codex')"
    exit 1
}
