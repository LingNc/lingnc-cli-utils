#!/bin/bash

# ==========================================
# 多用户协作环境初始化脚本 (Debian)
# 功能：
# 1. 安装 Brew 和 Docker 的系统依赖
# 2. 拦截普通用户的 apt 命令
# 3. 创建协作共享目录
# 4. 预下载 Homebrew / Docker 资源到本地缓存
# ==========================================

# 检测是否为 Root 身份
if [ "$(id -u)" -ne 0 ]; then
    echo -e "\033[31m[错误] 请使用 Root 权限运行此脚本！\033[0m"
    echo "提示：请尝试使用 sudo bash $0 $@"
    exit 1
fi

set -e

# 定义共享组和目录
SHARE_GROUP="devteam"
SHARE_DIR="/home/share"
CACHE_DIR="/opt/dev-cache"
DOCKER_VERSION="29.1.4"

echo ">>> [1/5] 更新系统并安装基础依赖..."
apt-get update
# 安装编译环境、Rootless Docker 依赖 (uidmap, dbus-user-session)
apt-get install -y build-essential curl file git \
    uidmap dbus-user-session fuse-overlayfs iptables \
    slirp4netns

echo ">>> [2/5] 配置全局 Shell 环境 (拦截 apt, 预设路径)..."
# 创建一个全局 profile 脚本，所有用户登录时都会加载
cat > /etc/profile.d/99-dev-env.sh << 'EOF'
# 拦截 apt 和 apt-get
if [ "$(id -u)" -ne 0 ]; then
    apt() {
        echo -e "\033[31m[Permission Denied]\033[0m 你没有系统级软件管理权限."
        echo -e "   请使用 \033[32mbrew install <package>\033[0m 安装软件给自己使用。"
        echo -e "   或者联系管理员安装系统级依赖。"
    }
    apt-get() {
        apt "$@"
    }

    # 自动加载用户的 Homebrew (如果已安装)
    if [ -d "$HOME/.linuxbrew/bin" ]; then
        eval "$($HOME/.linuxbrew/bin/brew shellenv)"
    elif [ -d "/home/linuxbrew/.linuxbrew/bin" ]; then
        eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"
    fi

    # 自动设置 Docker Rootless Host
    if [ -S "$XDG_RUNTIME_DIR/docker.sock" ]; then
        export DOCKER_HOST="unix://$XDG_RUNTIME_DIR/docker.sock"
    fi
fi
EOF

echo ">>> [3/5] 创建共享协作目录..."
# 创建组
if ! getent group $SHARE_GROUP > /dev/null; then
    groupadd $SHARE_GROUP
fi

# 创建目录并设置权限
mkdir -p $SHARE_DIR
chown root:$SHARE_GROUP $SHARE_DIR
# 关键：chmod 2770
# 2 (SetGID): 在此目录下创建的新文件，自动继承父目录的组 ($SHARE_GROUP)
# 770: 属主和组可读写执行，其他人无权访问
chmod 2770 $SHARE_DIR

echo ">>> [4/5] 建立本地资源缓存..."
mkdir -p $CACHE_DIR

# 4.1 缓存 Homebrew (bare clone)
if [ ! -d "$CACHE_DIR/homebrew.git" ]; then
    echo "--> 正在下载 Homebrew 核心库 (bare clone)..."
    git clone --bare --depth=1 https://github.com/Homebrew/brew.git "$CACHE_DIR/homebrew.git"
else
    echo "--> Homebrew 缓存已存在，正在更新..."
    git --git-dir="$CACHE_DIR/homebrew.git" fetch origin master:master
fi

# [新增修复] 告诉系统级 Git 信任这个缓存目录，避免 "detected dubious ownership"
if ! git config --system --get-all safe.directory | grep -q "$CACHE_DIR/homebrew.git"; then
    echo "--> 配置 Git 全局信任列表..."
    git config --system --add safe.directory "$CACHE_DIR/homebrew.git"
fi

# 4.2 缓存 Docker 主程序 (包含 docker 客户端)
DOCKER_MAIN_FILE="docker-${DOCKER_VERSION}.tgz"
if [ ! -f "$CACHE_DIR/$DOCKER_MAIN_FILE" ]; then
    echo "--> 正在下载 Docker 主程序包 (v${DOCKER_VERSION})..."
    curl -L -o "$CACHE_DIR/$DOCKER_MAIN_FILE" \
        "https://download.docker.com/linux/static/stable/x86_64/${DOCKER_MAIN_FILE}"
else
    echo "--> Docker 主程序包缓存已存在。"
fi

# 4.3 缓存 Docker Rootless Binaries (extras)
DOCKER_EXTRAS_FILE="docker-rootless-extras-${DOCKER_VERSION}.tgz"
if [ ! -f "$CACHE_DIR/$DOCKER_EXTRAS_FILE" ]; then
    echo "--> 正在下载 Docker Rootless 扩展包 (v${DOCKER_VERSION})..."
    curl -L -o "$CACHE_DIR/$DOCKER_EXTRAS_FILE" \
        "https://download.docker.com/linux/static/stable/x86_64/${DOCKER_EXTRAS_FILE}"
else
    echo "--> Docker Rootless 扩展包缓存已存在。"
fi

chmod -R 755 $CACHE_DIR

echo ">>> [5/5] 系统初始化完成！缓存已就绪。"