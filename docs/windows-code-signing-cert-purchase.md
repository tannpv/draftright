# Windows code-signing cert — purchase & wire-up runbook

Unblocks the **direct-download installer** (`DraftRight-Setup-Windows-*.exe`),
which ships **unsigned** today so Smart App Control / SmartScreen block it (#154).
Store users are unaffected — the MSIX is Microsoft-signed. This runbook is for the
direct-download channel only.

## Read this first — two hard truths

### 1. OV signing does NOT guarantee a Smart App Control pass
SmartScreen ≠ Smart App Control (SAC). They are different gates:
- **SmartScreen** — "unknown publisher" warning. A signed installer fixes this
  (OV builds reputation over time; **EV gets instant reputation**).
- **Smart App Control** — Win11 ML/reputation gate, stricter. It can still block a
  freshly-signed, low-reputation app. **No OV/EV cert guarantees a SAC pass** —
  reputation accrues with install volume. The only *guaranteed* SAC bypass is the
  **Microsoft Store** (already shipped).

So a cert's real win = kills the "unknown publisher" scare on direct download and
starts building reputation. It is an improvement, not a silver bullet. If the goal
is "installs cleanly with zero warnings, guaranteed", that's the Store, not a cert.

### 2. Since June 2023 there is NO downloadable `.pfx` for OV/EV
CA/B Forum now mandates the private key live on **FIPS 140-2 hardware** — a USB
token **or a cloud HSM**. You cannot get a plain PFX file anymore.

**Consequence for our CI:** the dormant wiring assumes a PFX
(`WINDOWS_SIGNING_PFX_BASE64` secret in `build-windows.yml` +
`installer/sign-file.ps1`). A USB token cannot plug into GitHub Actions. So to
keep CI signing, buy a cert with a **cloud-signing service** (eSigner / KeyLocker)
and rework `sign-file.ps1` to call its CLI tool instead of loading a PFX. A USB
token only works if you sign **locally** on a Windows box each release.

## Decision — what to buy

| Option | Cost/yr | SmartScreen | CI-signable | Buy if |
|---|---|---|---|---|
| **OV + cloud signing (recommended)** | ~$200–350 | reputation builds over time | ✅ via eSigner/KeyLocker | want CI signing, lowest cost |
| EV + cloud signing | ~$300–600 | **instant** reputation | ✅ | want zero SmartScreen warnings day 1 |
| OV/EV + USB token | ~$200–400 + token | as above | ❌ (local sign only) | fine signing manually each release |

Recommended: **OV + cloud signing** (e.g. SSL.com OV + eSigner — cheapest VN-eligible
cloud path; DigiCert KeyLocker and Sectigo/GlobalSign via a reseller also work).
Step up to **EV** if the "unknown publisher" warning on day 1 is unacceptable and
the extra cost is fine.

## Eligibility (Vietnam)
- **OV org validation** — issued against a registered business. Tan's **hộ kinh
  doanh** (household business registration) is the validating entity. Have the
  registration certificate + a verifiable business phone + address ready.
- Individual OV is also possible (validated against government ID) but org-validated
  reads more trustworthy to users.
- Azure Trusted Signing is **NOT** an option — not available for Vietnam (#154).

## Purchase steps
1. **Choose** provider + OV + **cloud-signing** add-on (recommend SSL.com OV +
   eSigner). Confirm at checkout the key is cloud-HSM, not USB.
2. **Order** the cert; create the account.
3. **Validation docs** — submit hộ kinh doanh registration; complete the phone /
   business-address verification the CA calls. VN org validation is often manual:
   **budget 1–5 business days**, sometimes longer for a first issuance.
4. **Provision the key** in the provider's cloud HSM (eSigner/KeyLocker) once
   validated. The cert never leaves their HSM.
5. **Collect signing credentials** — for eSigner: username, password, **TOTP/OTP
   secret** (for automated 2FA), and the **credential ID**. KeyLocker: API key +
   host + cert alias.

## CI wire-up (after issuance)
The current wiring is PFX-shaped and must change to cloud-signing:
1. Rework `DraftRightWindows/installer/sign-file.ps1` to invoke the provider's
   CLI (SSL.com **CodeSignTool** / DigiCert **smctl**) instead of importing a PFX.
   Sign both the app exe and the Inno Setup installer output.
2. Replace the dormant secrets in `build-windows.yml`:
   - remove `WINDOWS_SIGNING_PFX_BASE64` + `WINDOWS_SIGNING_PFX_PASSWORD`
   - add the cloud-signer set — eSigner: `ESIGNER_USERNAME`, `ESIGNER_PASSWORD`,
     `ESIGNER_TOTP_SECRET`, `ESIGNER_CREDENTIAL_ID` (as GitHub Actions secrets).
3. Keep signing **gated on the secrets being present** (as it is now) so forks and
   secret-less runs still build unsigned — no hard failure.
4. **RULE #1:** the sign step is one chokepoint (`sign-file.ps1`) called by the
   build — do not inline signtool calls per artifact. One script signs exe +
   installer; adding the MSIX later reuses it.

## Verify a signed build
- CI: `signtool verify /pa /v DraftRight-Setup-Windows-<ver>-x64.exe` → "Successfully verified".
- Manual: installer **Properties → Digital Signatures** shows the hộ kinh doanh
  subject; running it shows a *named* publisher, not "Unknown publisher".
- Real test: on a clean Win11 box with SmartScreen on, the "unknown publisher"
  wall is gone. (SAC may still gate a low-reputation new cert — expected; reputation
  builds. Store remains the guaranteed-clean path.)

## After it works
- Update `DraftRightWindows/CLAUDE.md` signing section (cert type, provider, renewal
  date, that CI now signs) and `docs/release-runbook.md`.
- Renewal: OV/EV are 1–3 yr; **calendar the expiry** — an expired cert silently
  reverts direct download to unsigned.
- Store channel is unchanged and still the recommended install on the website.
