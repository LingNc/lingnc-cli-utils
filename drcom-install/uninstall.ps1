$ErrorActionPreference = 'Stop'

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)
}

if (-not (Test-Administrator)) {
    throw '请以管理员权限运行 uninstall.bat。'
}

$taskName = 'Dogcom'
$installDir = 'C:\ProgramData\Dogcom'

try {
    $task = Get-ScheduledTask -TaskName $taskName -ErrorAction Stop
    if ($null -ne $task) {
        Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
    }
}
catch {
    # 任务不存在时忽略
}

if (Test-Path $installDir) {
    Remove-Item -Path $installDir -Recurse -Force
}

Write-Host '卸载完成: 已删除 Dogcom 计划任务与安装目录。'
