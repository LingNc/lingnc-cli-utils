#!/bin/bash

# ==========================================
# 用户环境加载脚本 (修复版 v3.0)
# 功能：
# 1. 将用户加入共享组并创建链接
# 2. 配置 subuid/subgid (Docker 需要)
# 3. 切换到用户身份自动安装 Brew 和 Rootless Docker
# 修复：Systemd 连接竞态，增加等待逻辑
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
CACHE_DIR="/opt/dev-cache"

# 自动寻找缓存的 Docker 包（主程序 + Rootless 扩展）
DOCKER_MAIN=$(find $CACHE_DIR -name "docker-*.tgz" | grep -v rootless | sort -V | tail -n 1)
DOCKER_EXTRAS=$(find $CACHE_DIR -name "docker-rootless-extras-*.tgz" | sort -V | tail -n 1)

# 检查参数
if [ -z "$TARGET_USER" ]; then
    echo "用法: sudo ./user_onboard.sh <username>"
    exit 1
fi

# 检查缓存
if [ ! -d "$CACHE_DIR/homebrew.git" ] || [ -z "$DOCKER_MAIN" ] || [ -z "$DOCKER_EXTRAS" ]; then
    echo -e "\033[31m[错误] 未找到本地缓存！\033[0m"
    echo "请先运行 setup_server.sh 进行系统初始化和缓存下载。"
    exit 1
fi

# 1. 用户创建与权限
if ! id "$TARGET_USER" &>/dev/null; then
    echo "用户 $TARGET_USER 不存在，正在创建..."
    useradd -m -s /bin/bash "$TARGET_USER"
    echo "请为用户设置密码："
    passwd "$TARGET_USER"
fi

# 预先开启 linger，确保用户级 systemd 可用（先重置再启用，避免残留状态）
echo ">>> [0/3] 初始化用户 linger..."
loginctl disable-linger "$TARGET_USER" >/dev/null 2>&1
loginctl enable-linger "$TARGET_USER"
sleep 2

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
sudo -u "$TARGET_USER" -i DOCKER_MAIN="$DOCKER_MAIN" DOCKER_EXTRAS="$DOCKER_EXTRAS" CACHE_DIR="$CACHE_DIR" bash << 'EOF'
    # ----------------------------------------
    # 以下命令以 目标用户 身份运行
    # ----------------------------------------

    set -e

    # 补充 systemd 相关环境，避免 rootless 安装误判
    export XDG_RUNTIME_DIR=/run/user/$(id -u)
    export DBUS_SESSION_BUS_ADDRESS=unix:path=${XDG_RUNTIME_DIR}/bus

    # 等待 systemd bus socket 就绪，解决连接被拒绝问题
    echo "--> 等待 Systemd Bus Socket 就绪..."
    TIMEOUT=0
    while [ ! -S "${XDG_RUNTIME_DIR}/bus" ]; do
        if [ $TIMEOUT -gt 10 ]; then
            echo "错误: Systemd 用户服务启动超时！"
            echo "调试: $(ls -ld ${XDG_RUNTIME_DIR} 2>&1)"
            exit 1
        fi
        sleep 1
        echo "    ... (${TIMEOUT}s)"
        TIMEOUT=$((TIMEOUT+1))
    done
    echo "--> Systemd 已连接。"

    # 1. 安装 Homebrew (使用本地缓存)
    # 使用 Git clone 方式，避免需要额外权限和外网
    if [ ! -d "$HOME/.linuxbrew" ]; then
        echo "--> 正在为用户安装 Homebrew (缓存克隆)..."
        git clone "$CACHE_DIR/homebrew.git" "$HOME/.linuxbrew"
        git -C "$HOME/.linuxbrew" remote set-url origin https://github.com/Homebrew/brew
        eval "$($HOME/.linuxbrew/bin/brew shellenv)"

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

    if ! systemctl --user is-active --quiet docker; then
        # 解压主程序（包含 docker 客户端）
        tar -xzf "$DOCKER_MAIN" -C "$HOME/bin" --strip-components=1
        # 解压 rootless 扩展
        tar -xzf "$DOCKER_EXTRAS" -C "$HOME/bin" --strip-components=1

        if command -v dockerd-rootless-setuptool.sh > /dev/null 2>&1; then
            dockerd-rootless-setuptool.sh install --skip-iptables
        else
            echo "dockerd-rootless-setuptool.sh 未找到，安装失败" >&2
            exit 1
        fi

        if ! grep -q "DOCKER_HOST" "$HOME/.bashrc"; then
            echo '# Docker Rootless 配置' >> "$HOME/.bashrc"
            echo 'export PATH=$HOME/bin:$PATH' >> "$HOME/.bashrc"
            echo "export DOCKER_HOST=unix://${XDG_RUNTIME_DIR}/docker.sock" >> "$HOME/.bashrc"
        fi

        systemctl --user enable docker
        systemctl --user start docker
    else
        echo "--> Docker 已在运行，跳过安装。"
    fi

    echo "--> 用户配置完成！"
EOF

echo "=== 完成 ==="