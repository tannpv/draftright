# Install/uninstall the DraftRight backend scheduled task
# Usage: powershell -ExecutionPolicy Bypass -File install-service.ps1 [-Action install|uninstall|status]

param(
    [ValidateSet("install", "uninstall", "status")]
    [string]$Action = "install"
)

$TaskName = "DraftRight Backend"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$ScriptPath = Join-Path $ScriptDir "start-server.ps1"

switch ($Action) {
    "install" {
        # Remove existing task if any
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue

        $taskAction = New-ScheduledTaskAction `
            -Execute "powershell.exe" `
            -Argument "-ExecutionPolicy Bypass -WindowStyle Hidden -File `"$ScriptPath`""

        # Trigger: at logon + every 2 minutes repeating
        $logonTrigger = New-ScheduledTaskTrigger -AtLogOn
        $logonTrigger.Repetition = (New-ScheduledTaskTrigger -Once -At "00:00" `
            -RepetitionInterval (New-TimeSpan -Minutes 2)).Repetition

        $settings = New-ScheduledTaskSettingsSet `
            -AllowStartIfOnBatteries `
            -DontStopIfGoingOnBatteries `
            -StartWhenAvailable `
            -ExecutionTimeLimit (New-TimeSpan -Minutes 5)

        Register-ScheduledTask `
            -TaskName $TaskName `
            -Action $taskAction `
            -Trigger $logonTrigger `
            -Settings $settings `
            -Description "Ensures DraftRight backend Docker services are running" `
            -RunLevel Limited

        Write-Host "Installed scheduled task: $TaskName" -ForegroundColor Green
        Write-Host "  Runs at logon + every 2 minutes"
    }
    "uninstall" {
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
        Write-Host "Uninstalled scheduled task: $TaskName" -ForegroundColor Green
    }
    "status" {
        $task = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
        if ($task) {
            Write-Host "$TaskName is registered (State: $($task.State))" -ForegroundColor Green
            $info = Get-ScheduledTaskInfo -TaskName $TaskName
            Write-Host "  Last run: $($info.LastRunTime)"
            Write-Host "  Last result: $($info.LastTaskResult)"
        } else {
            Write-Host "$TaskName is not registered" -ForegroundColor Yellow
        }
    }
}
