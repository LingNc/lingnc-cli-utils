#!/bin/bash

# ==========================================
# 用户环境加载脚本
# 功能：
# 1. 将用户加入共享组并创建链接
# 2. 配置 subuid/subgid (Docker 需要)
# 3. 切换到用户身份自动安装 Brew 和 Rootless Docker
# ==========================================

# 检测是否为 Root 身份
if [ "$(id -u)" -ne 0 ]; then
    echo -e "\033[31m[错误] 请使用 Root 权限运行此脚本！\033[0m"
    echo "提示：请尝试使用 sudo bash $0 $@"
    exit 1
fi

TARGET_USER=$1
SHARE_GROUP="devteam"
SHARE_DIR="/home/share"

# 检查参数
if [ -z "$TARGET_USER" ]; then
    echo "用法: sudo ./user_onboard.sh <username>"
    exit 1
fi

# 1. 用户创建与权限
if ! id "$TARGET_USER" &>/dev/null; then
    echo "用户 $TARGET_USER 不存在，正在创建..."
    useradd -m -s /bin/bash "$TARGET_USER"
    echo "请为用户设置密码："
    passwd "$TARGET_USER"
fi

# 预先开启 linger，确保用户级 systemd 可用
echo ">>> [0/3] 启用用户 linger..."
loginctl enable-linger "$TARGET_USER"
sleep 1

echo ">>> [1/3] 配置用户权限和共享目录..."
# 添加用户到共享组
usermod -aG $SHARE_GROUP "$TARGET_USER"

# 创建软链接 (share-workspace)
USER_HOME=$(eval echo "~$TARGET_USER")
if [ ! -e "$USER_HOME/share-workspace" ]; then
    ln -s $SHARE_DIR "$USER_HOME/share-workspace"
    chown -h "$TARGET_USER":"$TARGET_USER" "$USER_HOME/share-workspace"
fi

echo ">>> [2/3] 配置 Rootless Docker 命名空间映射..."
# 检查是否已有映射，如果没有则自动添加 (subuid/subgid)
# 通常 Debian 的 adduser 会自动处理，但手动确保一下
if ! grep -q "^$TARGET_USER:" /etc/subuid; then
    echo "$TARGET_USER:100000:65536" >> /etc/subuid
fi
if ! grep -q "^$TARGET_USER:" /etc/subgid; then
    echo "$TARGET_USER:100000:65536" >> /etc/subgid
fi

echo ">>> [3/3] 切换身份为 $TARGET_USER 进行软件安装..."
# 使用 sudo -u -i 模拟用户登录环境执行安装
sudo -u "$TARGET_USER" -i bash << 'EOF'
    # ----------------------------------------
    # 以下命令以 目标用户 身份运行
    # ----------------------------------------

    set -e

    # 补充 systemd 相关环境，避免 rootless 安装误判
    export XDG_RUNTIME_DIR=/run/user/$(id -u)
    export DBUS_SESSION_BUS_ADDRESS=unix:path=${XDG_RUNTIME_DIR}/bus

    # 1. 安装 Homebrew (如果不存在)
    # 使用 Git clone 方式，避免需要额外权限
    if [ ! -d "$HOME/.linuxbrew" ]; then
        echo "--> 正在为用户安装 Homebrew (git clone)..."
        git clone https://github.com/Homebrew/brew "$HOME/.linuxbrew"
        eval "$($HOME/.linuxbrew/bin/brew shellenv)"
        brew update --force --quiet

        if ! grep -q "brew shellenv" "$HOME/.bashrc"; then
            echo '# Homebrew 配置' >> "$HOME/.bashrc"
            echo "eval \"\$($HOME/.linuxbrew/bin/brew shellenv)\"" >> "$HOME/.bashrc"
        fi
    else
        eval "$($HOME/.linuxbrew/bin/brew shellenv)"
        echo "--> Homebrew 已安装，跳过。"
    fi

    # 2. 安装/启动 Rootless Docker（不重复安装系统 Docker 客户端）
    echo "--> 配置 Rootless Docker..."
    mkdir -p $HOME/bin
    export PATH=$HOME/bin:$PATH

    # 官方脚本会自动检测并安装 dockerd-rootless
    curl -fsSL https://get.docker.com/rootless | sh

    # 将 rootless 运行所需 PATH/DOCKER_HOST 持久化
    if ! grep -q "DOCKER_HOST" "$HOME/.bashrc"; then
        echo '# Docker Rootless 配置' >> "$HOME/.bashrc"
        echo 'export PATH=/home/linuxbrew/.linuxbrew/bin:$PATH' >> "$HOME/.bashrc"
        echo 'export PATH=$HOME/bin:$PATH' >> "$HOME/.bashrc"
        echo "export DOCKER_HOST=unix://${XDG_RUNTIME_DIR}/docker.sock" >> "$HOME/.bashrc"
    fi

    # 3. 设置 Docker 开机自启 (用户级 systemd)
    # 注意：这需要系统开启 lingering
    systemctl --user enable docker
    systemctl --user start docker

    echo "--> 用户配置完成！"
EOF

echo "=== 完成 ==="