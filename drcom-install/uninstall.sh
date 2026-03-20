#!/usr/bin/env bash

set -euo pipefail

SERVICE_NAME="dogcom"
INSTALL_BIN="/usr/local/bin/dogcom"
WRAPPER_BIN="/usr/local/bin/dogcom-wrapper.sh"
CONFIG_DIR="/etc/dogcom"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

if [[ "${EUID}" -ne 0 ]]; then
  echo "请使用 root 权限运行: sudo ./uninstall.sh"
  exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
  echo "错误: 当前系统缺少 systemd(systemctl)，无法按本脚本方式卸载。"
  exit 1
fi

echo "=== Dogcom 卸载 ==="

if systemctl list-unit-files | grep -q "^${SERVICE_NAME}\.service"; then
  systemctl stop "$SERVICE_NAME" 2>/dev/null || true
  systemctl disable "$SERVICE_NAME" 2>/dev/null || true
fi

rm -f "$SERVICE_FILE"
systemctl daemon-reload

rm -f "$INSTALL_BIN"
rm -f "$WRAPPER_BIN"
rm -rf "$CONFIG_DIR"

echo "卸载完成。"
echo "可执行检查: systemctl status dogcom"
