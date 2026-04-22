param(
    [Parameter(Mandatory = $true)]
    [string]$Username,
    [Parameter(Mandatory = $true)]
    [string]$Password
)

$ErrorActionPreference = 'Stop'

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)
}

if (-not (Test-Administrator)) {
    throw '请以管理员权限运行 install.bat。'
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$sourceExe = Join-Path $scriptDir 'dogcom.exe'
if (-not (Test-Path $sourceExe)) {
    $sourceExe = Join-Path $scriptDir 'dogcom'
}

$templateConf = Join-Path $scriptDir 'drcom.conf'
if (-not (Test-Path $sourceExe)) {
    throw "未找到 dogcom 可执行文件: $sourceExe"
}
if (-not (Test-Path $templateConf)) {
    throw "未找到配置模板: $templateConf"
}

$installDir = 'C:\ProgramData\Dogcom'
$targetExe = Join-Path $installDir 'dogcom.exe'
$targetConf = Join-Path $installDir 'drcom.conf'
$wrapperScript = Join-Path $installDir 'dogcom-wrapper.ps1'
$logFile = Join-Path $installDir 'dogcom.log'

New-Item -Path $installDir -ItemType Directory -Force | Out-Null

$ipCandidates = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction Stop |
    Where-Object {
        $_.IPAddress -ne '127.0.0.1' -and
        $_.ValidLifetime -ne ([TimeSpan]::Zero) -and
        $_.InterfaceAlias -notmatch 'Loopback'
    }

if (-not $ipCandidates) {
    throw '无法自动识别可用 IPv4 地址。'
}

$selectedIp = $ipCandidates | Sort-Object -Property InterfaceMetric, SkipAsSource | Select-Object -First 1
$adapter = Get-NetAdapter -InterfaceIndex $selectedIp.InterfaceIndex -ErrorAction Stop

$hostIp = $selectedIp.IPAddress
$hostName = $env:COMPUTERNAME
$hostOs = 'Windows'
$macRaw = $adapter.MacAddress
$macHex = ($macRaw -replace '[-:]', '').ToLowerInvariant()
if ($macHex.Length -ne 12) {
    throw "MAC 地址格式异常: $macRaw"
}

# 根据 dogcom 的 configparse 逻辑，mac 推荐使用 0x + 12位十六进制格式。
$macDogcom = "0x$macHex"

$confTemplate = Get-Content -Raw -Path $templateConf
$renderedConf = $confTemplate
$renderedConf = $renderedConf.Replace('__USERNAME__', $Username)
$renderedConf = $renderedConf.Replace('__PASSWORD__', $Password)
$renderedConf = $renderedConf.Replace('__HOST_IP__', $hostIp)
$renderedConf = $renderedConf.Replace('__HOSTNAME__', $hostName)
$renderedConf = $renderedConf.Replace('__MAC_ADDRESS__', $macDogcom)
$renderedConf = $renderedConf.Replace('__HOST_OS__', $hostOs)

Copy-Item -Path $sourceExe -Destination $targetExe -Force
Set-Content -Path $targetConf -Value $renderedConf -Encoding ascii

$wrapperContent = @"
`$ErrorActionPreference = 'Stop'
`$dogcomExe = 'C:\ProgramData\Dogcom\dogcom.exe'
`$dogcomConf = 'C:\ProgramData\Dogcom\drcom.conf'
`$logFile = 'C:\ProgramData\Dogcom\dogcom.log'
`$filterRegex = '^\[Keepalive[0-9A-Za-z_]+ (sent|recv)\]'

& `$dogcomExe -m dhcp -e -c `$dogcomConf -v 2>&1 |
    ForEach-Object {
        `$line = `$_.ToString()
        if (`$line -notmatch `$filterRegex) {
            Add-Content -Path `$logFile -Value `$line
        }
    }
"@

Set-Content -Path $wrapperScript -Value $wrapperContent -Encoding ascii

$action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument '-NoProfile -ExecutionPolicy Bypass -File "C:\ProgramData\Dogcom\dogcom-wrapper.ps1"'
$trigger = New-ScheduledTaskTrigger -AtStartup
$settings = New-ScheduledTaskSettingsSet -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit ([TimeSpan]::Zero)
$principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -RunLevel Highest -LogonType ServiceAccount

Register-ScheduledTask -TaskName 'Dogcom' -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force | Out-Null
Start-ScheduledTask -TaskName 'Dogcom'

Write-Host '安装成功: Dogcom 已注册为开机自启动任务 (SYSTEM)。'
Write-Host '查看任务: schtasks /Query /TN Dogcom /V /FO LIST'
Write-Host '查看日志: type C:\ProgramData\Dogcom\dogcom.log'
