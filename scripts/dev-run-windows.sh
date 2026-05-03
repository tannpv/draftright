#!/bin/bash
# Build and run DraftRight Windows on the UTM ARM64 VM in one command.
#
# Workflow:
#   1. Auto-commits any uncommitted changes (as a WIP commit on develop)
#      and pushes to origin so the VM can pull them.
#   2. Via WinRM: pulls latest develop, kills any running DraftRight
#      instance, runs `dotnet build` (Debug, ~20 sec — much faster than
#      `dotnet publish`), and launches the resulting EXE.
#   3. The app window appears on the VM screen. Watch it via the UTM
#      console.
#
# Usage:
#   ./scripts/dev-run-windows.sh           # auto-WIP commit + push + build + run
#   ./scripts/dev-run-windows.sh --no-push # skip git, assume code is already on VM
#
# To clean up auto-WIP commits later:
#   git log --oneline --grep '^wip: dev iteration'
#   git rebase -i HEAD~N   # squash the wip commits into a single feat: commit
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

if [[ "${1:-}" != "--no-push" ]]; then
    if [[ -n "$(git status --short)" ]]; then
        echo "→ auto-WIP commit on $(git branch --show-current)"
        git add -A
        git commit --no-verify -m "wip: dev iteration $(date -u +%Y-%m-%dT%H:%M:%SZ)" >/dev/null
    fi
    if [[ -n "$(git log @{u}..)" ]]; then
        echo "→ pushing"
        git push origin "$(git branch --show-current)" --quiet
    fi
fi

echo "→ pull + build + launch on VM..."
python3 - <<'PY'
import winrm
session = winrm.Session('http://192.168.64.5:5985/wsman', auth=('tan','123'), transport='basic')
session.protocol.transport.read_timeout_sec = 300

script = r"""
$env:Path = "C:\Program Files\dotnet;C:\Program Files\Git\cmd;$env:Path"
cd C:\draftright
git fetch origin 2>&1 | Out-Null
git reset --hard origin/develop 2>&1 | Out-Null
cd DraftRightWindows\DraftRightWindows

# Kill any running instance so we get the new build
Stop-Process -Name DraftRightWindows -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 500

# Debug build for ARM64. Much faster than `dotnet publish`:
#  - no PublishSingleFile bundling
#  - no self-contained runtime copy
#  - no IncludeAllContentForSelfExtract
$out = dotnet build -c Debug -r win-arm64 --self-contained false 2>&1
$errs = $out | Select-String -Pattern 'error '
if ($errs) {
    'BUILD ERRORS:'
    $errs | Select-Object -First 10
    exit 1
}

$exe = Get-ChildItem -Recurse -Path bin\arm64\Debug -Filter DraftRightWindows.exe -ErrorAction SilentlyContinue | Sort-Object LastWriteTime -Descending | Select-Object -First 1
if (-not $exe) {
    'BUILD OUTPUT NOT FOUND'
    exit 1
}

"Built: $($exe.FullName)"
"Size:  $([math]::Round($exe.Length/1MB, 1)) MB"

# Launch in interactive Session 1 (the user's desktop), not Session 0 (services).
# Plain Start-Process from a WinRM-Service context spawns into Session 0
# where tray icons / WinUI windows are invisible to the logged-in user.
# Using Task Scheduler with /IT (interactive) /RU 'tan' /RL HIGHEST forces Session 1.
'Launching via Task Scheduler into Session 1...'
$taskName = 'DraftRight-DevRun'
schtasks /Delete /TN $taskName /F 2>&1 | Out-Null
schtasks /Create /TN $taskName /SC ONCE /ST 00:00 /RL HIGHEST /TR "`"$($exe.FullName)`"" /F /IT /RU 'tan' 2>&1 | Out-Null
schtasks /Run /TN $taskName 2>&1 | Out-Null
Start-Sleep -Seconds 2
schtasks /Delete /TN $taskName /F 2>&1 | Out-Null

$proc = Get-Process DraftRightWindows -ErrorAction SilentlyContinue | Select-Object -First 1
if ($proc) {
    "Running: PID $($proc.Id) in SessionId $($proc.SessionId)"
    if ($proc.SessionId -eq 1) {
        'OK — tray icon should be visible on the VM.'
    } else {
        "WARN — process is in Session $($proc.SessionId), not 1. Tray icon may be invisible."
    }
} else {
    'WARN — process not detected after launch.'
}
"""
r = session.run_ps(script)
print(r.std_out.decode())
PY
