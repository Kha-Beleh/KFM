param([switch]$Deploy)
$ErrorActionPreference = 'Stop'
if (-not $env:KFM_CI_DB_PASSWORD) {
    $bytes = [byte[]]::new(24)
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
    $env:KFM_CI_DB_PASSWORD = [Convert]::ToHexString($bytes)
}
if ($env:KFM_CI_DB_PASSWORD -notmatch '^[A-Za-z0-9]{12,72}$') {
    throw 'KFM_CI_DB_PASSWORD must contain 12-72 letters and digits'
}
$deployValue = if ($Deploy) { 'true' } else { 'false' }
act workflow_dispatch -P ubuntu-22.04=ghcr.io/catthehacker/ubuntu:act-22.04 --input "deploy=$deployValue" --secret KFM_CI_DB_PASSWORD --env-file ci/act.env --secret-file ci/act.env --artifact-server-path .artifacts --cache-server-path .act-cache
exit $LASTEXITCODE
