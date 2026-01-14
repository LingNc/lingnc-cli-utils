# 多用户协作环境自动化脚本

这是一套在 Debian 服务器上快速部署多人开发环境的脚本。

主要功能：
1. 自动安装系统基础依赖。
2. 建立本地缓存（Homebrew 和 Docker），加速用户创建。
3. 为普通用户配置无 Root 权限的 Homebrew 和 Rootless Docker。
4. 自动创建共享协作目录。

## 包含文件

- setup_server.sh: 服务器初始化脚本（只运行一次，用于下载缓存和安装依赖）。
- user_onboard.sh: 用户创建脚本（每次有新人加入时运行）。
- check_user.sh: 环境检测脚本（用于验证用户环境是否正常）。
- set_workspace.sh: 适合有sudo的管理用户，只设置用户组和共享文件夹（用于管理和维护）。

## 使用步骤

1. 初始化服务器（仅需一次）
下载缓存文件并安装系统依赖：
sudo bash setup_server.sh

2. 添加新用户
自动创建用户并从本地缓存配置环境（例如添加用户 zhangsan）：
sudo bash user_onboard.sh zhangsan

3. 检查用户环境
验证 Homebrew 和 Docker 是否启动正常：
sudo bash check_user.sh zhangsan

## 注意事项
- 脚本必须使用 sudo 运行。
- 用户数据和 Homebrew 安装在用户的主目录下。
- 共享文件夹位于 ~/share-workspace。