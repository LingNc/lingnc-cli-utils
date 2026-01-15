1. 计划将各个脚本功能合并提供一个统一的命令指定，使用 check 等参数来控制使用不同的功能，提供install/uninstall安装到系统环境中。
2. 提供切换docker版本更新的时候移除之前的docker下载缓存重新更新等功能，还有更新brew仓库等功能。
3. 对于各个功能默认是全部安装，但是提供可以选择安装的方式比如安装brew（就要带上apt拦截），链接共享目录，还有docker目前这三个功能可以单独安装和卸载还有从缓存更新的功能，支持私有用户自己操作进行安装或者卸载（但是对于brew不提供卸载）。
4. 对于某些功能的安装应该计划性的设定在bashrc或者profile中该配置文件的范围，比如docker的环境变量配置 === xxxxx（某用户名） docker rootless config === 结束的话是 === xxxxx(某用户名) docker config end === 类似的这样的其他的同理，保证党用户重复安装的时候，或者更新软件的时候让下一代软件可以识别到上一次配置的东西一并删除和替换为新的，保持不混乱，上面只是举例实际可以根据需要命名。
5. 操纵和查看模块状态。

```bash
# 名称
multiuser

# 帮助 帮助可以最外层多级帮助并且每个工具也可以有对应的帮助，如 multiuser sys help / multiuser create help ...，但是后面的都不支持 --/- 形式只有最外层支持
multiuser help|--help|-h
# 版本
multiuser version|--version|-v

# 安装/卸载工具本身 (PLAN 1) (Root)
sudo ./multiuser install  # 安装到 /usr/local/bin/multiuser
sudo multiuser uninstall  # 卸载工具，用户无法再继续更新和操纵，但是已经安装的用户软件如docker不受影响

# 系统级管理 (Root 权限) PLAN 2
# 初始化
sudo multiuser sys init          # (原 setup_server.sh) 下载缓存、装依赖
# 更新
sudo multiuser sys update        # (PLAN 2) 更新 Docker/Brew 缓存
sudo multiuser sys check         # 检查系统级依赖状态
sudo multiuser sys remove        # 移除系统级依赖

# 用户管理 (Root 权限) 初始化一键管理
sudo multiuser create <username> [options]  # (原 user_onboard.sh)
    --with-docker    # 选项：安装 Docker
    --with-brew      # 选项：安装 Brew
    --link-share     # 选项：链接共享目录
    --all            # 默认选项

# 用户操作 (Root) 默认 all
sudo multiuser config <username> install all|docker|brew|share # (PLAN 3) 安装指定模块
sudo multiuser config <username> update all|docker|brew|share # (PLAN 4) 刷新配置文件并检查更新所有模块
sudo multiuser config <username> remove all|docker|brew|share # 移除指定模块配置
sudo multiuser config <username> check all|docker|brew|share  # 检查指定模块配置状态（包含是否拦截的部分 同 check_user.sh）

# 用户自己操作 (无 Root)（默认非all必须用户自己的指定）
# 模块操纵(支持 Root 指定用户，或用户自己运行无需root)
multiuser module install all|docker|brew|share     # 单独安装模块
multiuser module remove all|docker|brew|share      # 单独卸载模块(如果是brew需要用户输入"我已经知晓卸载后会删除本地我自己安装的所有软件"，二次确认才可以卸载)
multiuser module update all|docker|brew|share      # 从缓存更新模块
multiuser module check all|docker|brew|share       # 检查模块状态

# docker状态查看和管理
multiuser start docker
multiuser stop docker
multiuser status docker
multiuser enable docker
multiuser disable docker

```