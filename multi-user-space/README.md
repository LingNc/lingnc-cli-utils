# 多用户协作环境自动化脚本

这是一套在 Debian 服务器上快速部署多人开发环境的脚本，现已统一为单文件 CLI：`multiuser`。

主要功能：
1. 自动安装系统基础依赖与缓存（Docker/Brew）。
2. 为普通用户配置无 Root 权限的 Homebrew 和 Rootless Docker。
3. 创建共享协作目录并维护软链。
4. 一条命令安装/更新/检查/列表/卸载模块，支持 Root 代理用户操作。

## 快速开始

1. **安装脚本到系统（Root）**
	```bash
	sudo ./multiuser install
	```

2. **初始化系统（Root）**
	```bash
	sudo multiuser sys init
	```

3. **检查系统依赖与缓存（Root）**
	```bash
	sudo multiuser sys check
	```

4. **为用户安装环境（Root）**
	- 新用户：
		```bash
		sudo multiuser create zhangsan --all
		```
	- 现有用户：
		```bash
		sudo multiuser config zhangsan install all
		```

5. **常用操作**
	- Root 代理：`sudo multiuser config zhangsan update docker`
	- 用户自助：`multiuser module check all`
	- 资产盘点：`multiuser module list`
	- 系统清单：`sudo multiuser sys list`
	- Docker 控制：`multiuser start docker`

## 主要命令

- 系统级（Root）：
  - `sys init|update|check|remove|list`
- 用户初始化（Root）：
  - `create <user> [--with-docker] [--with-brew] [--link-share] [--all]`
  - `config <user> install|update|remove|check|list [all|docker|brew|share]`
- 用户模块（可 Root 代理）：
  - `module install|update|remove|check|list [all|docker|brew|share] [--user <name>]`
- Docker 快捷：
  - `start|stop|status|enable|disable docker [--user <name>]`

## 兼容保留

历史脚本仍在仓库中（setup_server.sh、user_onboard.sh 等），但推荐使用 `multiuser`。

### 旧脚本（保留）

- setup_server.sh: 服务器初始化脚本（只运行一次，用于下载缓存和安装依赖）。
- user_onboard.sh: 用户创建脚本（每次有新人加入时运行）。
- check_user.sh: 环境检测脚本（用于验证用户环境是否正常）。
- set_workspace.sh: 适合有 sudo 的管理用户，只设置用户组和共享文件夹（用于管理和维护）。

### 注意事项

- 脚本必须使用 sudo 运行（除用户自助命令）。
- 用户数据和 Homebrew 安装在用户主目录下。
- 共享目录位于 /home/share，用户侧软链为 ~/share-workspace。