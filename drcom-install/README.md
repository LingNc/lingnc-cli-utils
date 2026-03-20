# dogcom 一键安装与服务化

该目录用于把 dogcom 客户端自动安装为系统后台服务，并自动渲染配置文件。

## 目录说明

- `dogcom`: Linux 可执行文件（需具备可执行权限）
- `drcom.conf`: 配置模板（包含占位符）
- `install.sh`: Linux 一键安装脚本（systemd）
- `install.bat`: Windows 一键安装入口
- `install.ps1`: Windows 安装逻辑（配置渲染 + 计划任务）
- `uninstall.sh`: Linux 卸载脚本
- `uninstall.bat`: Windows 卸载入口
- `uninstall.ps1`: Windows 卸载逻辑

## Linux 安装

1. 确保当前目录内有 `dogcom` 与 `drcom.conf`
2. 赋予脚本权限:

```bash
chmod +x ./install.sh
```

3. 以 root 执行:

```bash
sudo ./install.sh
```

安装完成后会生成:

- 配置文件: `/etc/dogcom/drcom.conf`
- 主程序: `/usr/local/bin/dogcom`
- 日志过滤包装器: `/usr/local/bin/dogcom-wrapper.sh`
- systemd 服务: `/etc/systemd/system/dogcom.service`

常用命令:

```bash
systemctl status dogcom
journalctl -u dogcom -f
systemctl restart dogcom
```

Linux 卸载:

```bash
sudo ./uninstall.sh
```

## Windows 安装

1. 确保当前目录内有 `dogcom.exe`（或 `dogcom`）和 `drcom.conf`
2. 右键“以管理员身份运行” `install.bat`
3. 按提示输入账号密码

安装完成后会生成:

- 安装目录: `C:\ProgramData\Dogcom`
- 配置文件: `C:\ProgramData\Dogcom\drcom.conf`
- 过滤包装器: `C:\ProgramData\Dogcom\dogcom-wrapper.ps1`
- 开机自启任务: `Dogcom`（SYSTEM）

常用命令:

```bat
schtasks /Query /TN Dogcom /V /FO LIST
schtasks /Run /TN Dogcom
type C:\ProgramData\Dogcom\dogcom.log
```

Windows 卸载:

```bat
uninstall.bat
```

## 日志过滤规则

默认会过滤以下报文行，避免 Keepalive 十六进制日志爆炸:

```regex
^\[Keepalive[0-9A-Za-z_]+ (sent|recv)\]
```

但会保留关键状态日志，例如登录成功信息、`Keepalive in loop.` 等。

## 验收建议

1. 安装后无需手改配置，服务应能直接启动。
2. 杀死 dogcom 进程后，systemd 应在约 5 秒内拉起（Linux）。
3. 查看日志时不应出现 `^[Keepalive... sent|recv]` 十六进制包。
4. 系统重启后应自动运行。
