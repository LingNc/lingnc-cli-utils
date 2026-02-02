#!/bin/bash

# 定义颜色，让输出更清晰
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 检查是否以 root 权限运行
if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}请使用 root 权限运行此脚本 (例如: sudo ./npc-install.sh)${NC}"
  exit 1
fi

# 1. 检查当前目录下是否有 npc 可执行文件
if [ ! -f "./npc" ]; then
    echo -e "${RED}错误：当前目录下未找到 'npc' 文件。${NC}"
    echo "请确保你已经下载了解压了 npc 客户端，并且 npc-install.sh 和 npc 在同一目录下。"
    exit 1
fi

echo -e "${GREEN}正在准备安装环境...${NC}"

# 2. 创建目录并复制文件
INSTALL_DIR="/opt/npc"
mkdir -p "$INSTALL_DIR"

# 停止旧服务（如果存在），防止复制时文件被占用
if systemctl is-active --quiet npc; then
    echo "停止现有 npc 服务..."
    systemctl stop npc
fi

echo "正在复制 npc 到 $INSTALL_DIR ..."
cp -f ./npc "$INSTALL_DIR/npc"
chmod +x "$INSTALL_DIR/npc"

# 3. 获取用户输入的启动命令
echo "----------------------------------------------------------------"
echo -e "${GREEN}请输入完整的启动命令${NC} (例如: ./npc -server=xxx:8024 -vkey=xxx -type=tcp)"
echo "----------------------------------------------------------------"
read -p "命令: " USER_CMD

# 4. 解析参数
# 使用 sed 去除命令开头的 ./npc 或 npc 以及前面的空格，只保留参数部分
ARGS=$(echo "$USER_CMD" | sed -E 's/^(\.\/)?npc[[:space:]]*//')

if [ -z "$ARGS" ]; then
    echo -e "${RED}错误：未能解析到有效参数。请确保输入了完整的参数。${NC}"
    exit 1
fi

echo -e "解析到的参数为: ${GREEN}$ARGS${NC}"

# 5. 生成 Systemd 服务文件
SERVICE_FILE="/etc/systemd/system/npc.service"

cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=NPS Client Service
After=network.target syslog.target

[Service]
Type=simple
User=root
WorkingDirectory=$INSTALL_DIR
# 使用绝对路径启动，并填入用户输入的参数
ExecStart=$INSTALL_DIR/npc $ARGS
Restart=always
RestartSec=5
# 限制日志大小，防止占满磁盘
StandardOutput=syslog
StandardError=syslog

[Install]
WantedBy=multi-user.target
EOF

echo "已生成服务文件: $SERVICE_FILE"

# 6. 重新加载并启动服务
echo -e "${GREEN}正在配置系统服务...${NC}"
systemctl daemon-reload
systemctl enable npc
systemctl restart npc

# 7. 检查状态
sleep 2
if systemctl is-active --quiet npc; then
    echo "----------------------------------------------------------------"
    echo -e "${GREEN}安装成功！NPC 服务正在运行。${NC}"
    echo "----------------------------------------------------------------"
    echo "查看运行状态: systemctl status npc"
    echo "查看运行日志: journalctl -u npc -f"
    echo "停止服务:     systemctl stop npc"
    echo "重启服务:     systemctl restart npc"
else
    echo -e "${RED}警告：服务启动失败，请检查参数是否正确。${NC}"
    echo "请运行 'systemctl status npc' 查看详细错误信息。"
fi
