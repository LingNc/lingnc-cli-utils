#!/usr/bin/env bash

set -euo pipefail

SERVICE_NAME="dogcom"
INSTALL_BIN="/usr/local/bin/dogcom"
WRAPPER_BIN="/usr/local/bin/dogcom-wrapper.sh"
CONFIG_DIR="/etc/dogcom"
CONFIG_FILE="$CONFIG_DIR/drcom.conf"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_BIN="$SCRIPT_DIR/dogcom"
TEMPLATE_CONF="$SCRIPT_DIR/drcom.conf"

if [[ "${EUID}" -ne 0 ]]; then
  echo "请使用 root 权限运行: sudo ./install.sh"
  exit 1
fi

if [[ ! -f "$SOURCE_BIN" ]]; then
  echo "错误: 未找到 dogcom 可执行文件: $SOURCE_BIN"
  exit 1
fi

if [[ ! -f "$TEMPLATE_CONF" ]]; then
  echo "错误: 未找到配置模板: $TEMPLATE_CONF"
  exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
  echo "错误: 当前系统缺少 systemd(systemctl)，无法按本脚本方式安装。"
  exit 1
fi

if ! command -v ip >/dev/null 2>&1; then
  echo "错误: 未找到 ip 命令(iproute2)。"
  exit 1
fi

if ! command -v stdbuf >/dev/null 2>&1; then
  echo "错误: 未找到 stdbuf(coreutils)。"
  exit 1
fi

echo "=== Dogcom 自动安装 ==="
read -r -p "请输入校园网账号: " USERNAME
read -r -s -p "请输入校园网密码: " PASSWORD
echo

if [[ -z "$USERNAME" || -z "$PASSWORD" ]]; then
  echo "错误: 账号和密码不能为空。"
  exit 1
fi

DEFAULT_ROUTE_INFO="$(ip -4 route get 1.1.1.1 2>/dev/null || true)"
IFACE="$(awk '/ dev / {for(i=1;i<=NF;i++) if ($i=="dev") {print $(i+1); exit}}' <<<"$DEFAULT_ROUTE_INFO")"
HOST_IP="$(awk '/ src / {for(i=1;i<=NF;i++) if ($i=="src") {print $(i+1); exit}}' <<<"$DEFAULT_ROUTE_INFO")"

if [[ -z "$IFACE" ]]; then
  IFACE="$(ip -o link show up | awk -F': ' '{print $2}' | grep -v '^lo$' | head -n1 || true)"
fi

if [[ -z "$IFACE" ]]; then
  echo "错误: 无法自动识别可用网卡。"
  exit 1
fi

if [[ -z "$HOST_IP" ]]; then
  HOST_IP="$(ip -4 -o addr show dev "$IFACE" scope global | awk '{print $4}' | cut -d/ -f1 | head -n1 || true)"
fi

if [[ -z "$HOST_IP" ]]; then
  echo "错误: 无法识别网卡 $IFACE 的 IPv4 地址。"
  exit 1
fi

if [[ ! -r "/sys/class/net/$IFACE/address" ]]; then
  echo "错误: 无法读取网卡 $IFACE 的 MAC 地址。"
  exit 1
fi

RAW_MAC="$(cat "/sys/class/net/$IFACE/address")"
MAC_HEX="$(tr -d ':-' <<<"$RAW_MAC" | tr '[:upper:]' '[:lower:]')"

if [[ ! "$MAC_HEX" =~ ^[0-9a-f]{12}$ ]]; then
  echo "错误: MAC 地址格式异常: $RAW_MAC"
  exit 1
fi

# dogcom 配置中的 mac 字段使用 0x + 12 位十六进制连续格式。
MAC_DOGCOM="0x${MAC_HEX}"
HOST_NAME="$(hostname 2>/dev/null || uname -n)"
HOST_OS="Linux"

echo "检测到网卡: $IFACE"
echo "检测到 IP: $HOST_IP"
echo "检测到 MAC: $MAC_DOGCOM"
echo "检测到主机名: $HOST_NAME"

escape_sed() {
  sed -e 's/[\/&]/\\&/g' <<<"$1"
}

ESC_USERNAME="$(escape_sed "$USERNAME")"
ESC_PASSWORD="$(escape_sed "$PASSWORD")"
ESC_HOST_IP="$(escape_sed "$HOST_IP")"
ESC_HOST_NAME="$(escape_sed "$HOST_NAME")"
ESC_MAC="$(escape_sed "$MAC_DOGCOM")"
ESC_HOST_OS="$(escape_sed "$HOST_OS")"

mkdir -p "$CONFIG_DIR"

sed \
  -e "s/__USERNAME__/${ESC_USERNAME}/g" \
  -e "s/__PASSWORD__/${ESC_PASSWORD}/g" \
  -e "s/__HOST_IP__/${ESC_HOST_IP}/g" \
  -e "s/__HOSTNAME__/${ESC_HOST_NAME}/g" \
  -e "s/__MAC_ADDRESS__/${ESC_MAC}/g" \
  -e "s/__HOST_OS__/${ESC_HOST_OS}/g" \
  "$TEMPLATE_CONF" > "$CONFIG_FILE"

install -m 0755 "$SOURCE_BIN" "$INSTALL_BIN"

cat > "$WRAPPER_BIN" <<'EOF'
#!/usr/bin/env bash

set -euo pipefail

DOGCOM_BIN="/usr/local/bin/dogcom"
DOGCOM_CONF="/etc/dogcom/drcom.conf"

stdbuf -oL -eL "$DOGCOM_BIN" -m dhcp -e -c "$DOGCOM_CONF" -v 2>&1 \
  | grep --line-buffered -vE '^\[Keepalive[0-9A-Za-z_]+ (sent|recv)\]'
EOF
chmod 0755 "$WRAPPER_BIN"

cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=Dogcom Client Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$WRAPPER_BIN
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "$SERVICE_NAME" >/dev/null
systemctl restart "$SERVICE_NAME"

sleep 2
if systemctl is-active --quiet "$SERVICE_NAME"; then
  echo "安装成功: 服务已启动并设置为开机自启。"
  echo "查看状态: systemctl status dogcom"
  echo "跟踪日志: journalctl -u dogcom -f"
else
  echo "警告: 服务启动失败，请检查日志。"
  echo "排查命令: systemctl status dogcom --no-pager"
  echo "排查命令: journalctl -u dogcom -n 100 --no-pager"
  exit 1
fi
