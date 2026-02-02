# npc 安装与 systemd 服务脚本

此目录提供 npc 客户端的自动化安装脚本，脚本会把当前目录下的 npc 部署到 /opt/npc，并根据你输入的启动命令生成 systemd 服务。

## 功能

- 自动部署：复制 ./npc 到 /opt/npc 并赋予执行权限
- 智能解析：粘贴完整启动命令，自动提取参数
- 服务管理：生成 /etc/systemd/system/npc.service，启用并启动服务
- 覆盖更新：重复执行可更新二进制与启动参数

## 使用方式

1. 将 npc 可执行文件与脚本放在同一目录
2. 赋予脚本执行权限：
   - chmod +x npc-install.sh
3. 运行脚本：
   - sudo ./npc-install.sh
4. 按提示粘贴完整命令，例如：
   - ./npc -server=xxx:8024 -vkey=xxx -type=tcp

## 常用命令

- 查看状态：systemctl status npc
- 查看日志：journalctl -u npc -f
- 停止服务：systemctl stop npc
- 重启服务：systemctl restart npc

## 说明

- 脚本要求 root 权限执行。
- 服务默认使用 root 运行，工作目录为 /opt/npc。
