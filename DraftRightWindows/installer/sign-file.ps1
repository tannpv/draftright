<#
.SYNOPSIS
  Authenticode-sign a single file (the app .exe or the Inno Setup installer).

.DESCRIPTION
  Single source of truth for how DraftRight signs Windows binaries. Both the
  published app binary and the installer .exe are signed by calling THIS script
  with a different -Path — never by copy-pasting a signtool line into each CI
  step. Two copies of a signing command are two things that drift; that exact
  failure mode (a copy-pasted release step losing a field) is issue #22, where
  Windows shipped installers with no sha256 for two months.

  Signing is what unblocks the auto-updater end to end (#154): Smart App Control
  and enterprise WDAC refuse to LAUNCH an unsigned .exe, so the download +
  integrity check succeed and then the install fails at the last step.

  GATING — this script is deliberately a no-op until a cert exists:
    * Signing secret ABSENT  -> log "UNSIGNED (#154)" and exit 0. The build
      still produces the same unsigned installer it does today. This is the
      "wire now, cert later" state: the plumbing is in place, dormant.
    * Signing secret PRESENT  -> sign, then verify. Any failure is FATAL, so a
      misconfigured cert fails the release loudly instead of silently shipping
      an unsigned binary that Smart App Control will block on the user's machine.

  When a cert is provisioned, set the two repo secrets and nothing else changes:
      WINDOWS_SIGNING_PFX_BASE64    base64 of the .pfx (OV or EV file cert)
      WINDOWS_SIGNING_PFX_PASSWORD  the .pfx password

  SWAPPING TO AZURE TRUSTED SIGNING (the ~$10/mo path in #154): replace the
  Sign-WithPfx call below with a call to azure/trusted-signing-action in the
  workflow, or add a Sign-WithAzure branch here that dispatches on the presence
  of AZURE_* secrets. The gating contract (absent = skip, present = sign+verify)
  stays identical, so the workflow steps do not change.

.PARAMETER Path
  The file to sign. Must exist.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Path
)

$ErrorActionPreference = 'Stop'

# ── Signing constants (one source of truth) ────────────────────────────────
# RFC 3161 timestamp authority. Timestamping is what keeps a signature valid
# after the signing cert expires, so it is not optional. DigiCert's public
# RFC 3161 endpoint per their docs; swap only if the CA mandates its own.
$TimestampUrl = 'http://timestamp.digicert.com'
$DigestAlgo   = 'SHA256'   # SmartScreen/WDAC require SHA-256; SHA-1 is rejected.

# Env var names — referenced once, here, so a rename can't half-land.
$PfxBase64Var   = 'WINDOWS_SIGNING_PFX_BASE64'
$PfxPasswordVar = 'WINDOWS_SIGNING_PFX_PASSWORD'

function Resolve-SignTool {
    <# Locate signtool.exe on the runner. It ships with the Windows SDK but is
       not on PATH, so probe the known install roots and pick the newest. #>
    $cmd = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }

    $roots = @(
        "${env:ProgramFiles(x86)}\Windows Kits\10\bin",
        "${env:ProgramFiles}\Windows Kits\10\bin"
    )
    $found =
        $roots |
        Where-Object { $_ -and (Test-Path $_) } |
        ForEach-Object { Get-ChildItem -Path $_ -Recurse -Filter signtool.exe -ErrorAction SilentlyContinue } |
        # x64 signtool, newest SDK first.
        Where-Object { $_.FullName -match '\\x64\\' } |
        Sort-Object FullName -Descending |
        Select-Object -First 1

    if (-not $found) { throw "signtool.exe not found — is the Windows SDK installed on this runner?" }
    return $found.FullName
}

function Sign-WithPfx {
    param([string]$SignTool, [string]$Target, [string]$PfxBase64, [string]$PfxPassword)

    # Materialize the .pfx to a temp file; shred it in finally so the private
    # key never lingers on the runner disk after the step.
    $pfxPath = Join-Path ([System.IO.Path]::GetTempPath()) ("drsign-" + [System.Guid]::NewGuid().ToString('N') + ".pfx")
    try {
        [System.IO.File]::WriteAllBytes($pfxPath, [System.Convert]::FromBase64String($PfxBase64))

        & $SignTool sign `
            /fd $DigestAlgo `
            /f  $pfxPath `
            /p  $PfxPassword `
            /tr $TimestampUrl `
            /td $DigestAlgo `
            $Target
        if ($LASTEXITCODE -ne 0) { throw "signtool sign failed (exit $LASTEXITCODE) for $Target" }

        # Prove the signature is valid before we let the release proceed.
        & $SignTool verify /pa $Target
        if ($LASTEXITCODE -ne 0) { throw "signtool verify failed (exit $LASTEXITCODE) for $Target" }
    }
    finally {
        if (Test-Path $pfxPath) { Remove-Item -LiteralPath $pfxPath -Force -ErrorAction SilentlyContinue }
    }
}

# ── Main ───────────────────────────────────────────────────────────────────
if (-not (Test-Path -LiteralPath $Path)) {
    throw "sign-file.ps1: target does not exist: $Path"
}

$pfxBase64   = [System.Environment]::GetEnvironmentVariable($PfxBase64Var)
$pfxPassword = [System.Environment]::GetEnvironmentVariable($PfxPasswordVar)

if ([string]::IsNullOrWhiteSpace($pfxBase64)) {
    # Dormant path — no cert yet. Loud, but non-fatal by design (#154).
    Write-Host "::warning::UNSIGNED (#154): $PfxBase64Var not set — skipping code-signing of '$Path'. The installer will be blocked by Smart App Control on the user's machine until a cert is configured."
    exit 0
}
if ([string]::IsNullOrWhiteSpace($pfxPassword)) {
    throw "$PfxBase64Var is set but $PfxPasswordVar is empty — refusing to attempt an unsigned or half-configured sign."
}

$signTool = Resolve-SignTool
Write-Host "Signing '$Path' with signtool at '$signTool'"
Sign-WithPfx -SignTool $signTool -Target $Path -PfxBase64 $pfxBase64 -PfxPassword $pfxPassword
Write-Host "✓ Signed and verified: $Path"
