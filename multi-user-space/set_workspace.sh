#!/bin/bash

# 检测是否为 Root 身份
if [ "$(id -u)" -ne 0 ]; then
    echo -e "\033[31m[错误] 请使用 Root 权限运行此脚本！\033[0m"
    echo "提示：请尝试使用 sudo bash $0 $@"
    exit 1
fi

# 检查参数
TARGET_USER=$1
SHARE_GROUP="devteam"
SHARE_DIR="/home/share"
CACHE_DIR="/opt/dev-cache"

# 添加用户到共享组
usermod -aG $SHARE_GROUP "$TARGET_USER"

# 创建软链接 (share-workspace)
USER_HOME=$(eval echo "~$TARGET_USER")
if [ ! -e "$USER_HOME/share-workspace" ]; then
    ln -s $SHARE_DIR "$USER_HOME/share-workspace"
    chown -h "$TARGET_USER":"$TARGET_USER" "$USER_HOME/share-workspace"
fi