#!/bin/bash

# 检测是否为 Root 身份
if [ "$(id -u)" -ne 0 ]; then
    echo -e "\033[31m[错误] 请使用 Root 权限运行此脚本！\033[0m"
    echo "提示：请尝试使用 sudo bash $0 $@"
    exit 1
fi

TARGET_USER=$1

if [ -z "$TARGET_USER" ]; then
    echo "用法: sudo bash check_user.sh <username>"
    exit 1
fi

echo "=== 正在检查用户 $TARGET_USER 的开发环境 ==="

# 1. 检查 Homebrew
echo -n "[1/4] 检查 Homebrew... "
BREW_VERSION=$(sudo -u "$TARGET_USER" -i bash -c 'source ~/.bashrc; brew --version 2>/dev/null | head -n 1')
if [[ $BREW_VERSION == *"Homebrew"* ]]; then
    echo -e "\033[32m成功\033[0m ($BREW_VERSION)"
else
    echo -e "\033[31m失败\033[0m (未找到 brew 命令)"
fi

# 2. 检查 Docker 守护进程连接（不拉取外网镜像）
echo -n "[2/4] 检查 Docker 守护进程... "
DOCKER_INFO=$(sudo -u "$TARGET_USER" -i bash -c 'docker info 2>/dev/null')
if [ $? -eq 0 ]; then
    echo -e "\033[32m成功\033[0m (Daemon 响应正常)"
else
    echo -e "\033[31m失败\033[0m (无法连接 Docker Daemon)"
    USER_ID=$(id -u "$TARGET_USER")
    sudo -u "$TARGET_USER" XDG_RUNTIME_DIR=/run/user/$USER_ID systemctl --user status docker --no-pager | head -n 3
fi

# 3. 检查共享目录写权限
echo -n "[3/4] 检查共享目录权限... "
SHARE_TEST=$(sudo -u "$TARGET_USER" -i bash -c 'touch ~/share-workspace/.perm_check && rm ~/share-workspace/.perm_check && echo ok')
if [ "$SHARE_TEST" == "ok" ]; then
    echo -e "\033[32m成功\033[0m (可写)"
else
    echo -e "\033[31m失败\033[0m (无法写入 ~/share-workspace)"
fi

# 4. 检查 APT 拦截策略
echo -n "[4/4] 检查 APT 拦截策略... "

# 关键修改：加入 -i 参数强制进入交互模式，这样 .bashrc 才会完整加载
# 2>/dev/null 用于屏蔽因为没有真实 TTY 而产生的 "job control" 警告
APT_TYPE=$(sudo -u "$TARGET_USER" -i bash -i -c 'source ~/.bashrc; type -t apt' 2>/dev/null)

if [ "$APT_TYPE" = "function" ]; then
    echo -e "\033[32m成功\033[0m (已拦截)"
else
    echo -e "\033[31m失败\033[0m (未拦截)"
    # 同样加上 -i 参数来获取诊断信息
    APT_DESC=$(sudo -u "$TARGET_USER" -i bash -i -c 'source ~/.bashrc; type apt' 2>/dev/null)
    echo "      诊断信息: $APT_DESC"
    echo "      建议: 检查 user_onboard.sh 是否正确将拦截函数写入了 ~/.bashrc"
fi

echo "=== 检查结束 ==="