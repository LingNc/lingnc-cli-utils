## 分支策略
1. 拥有两个长期分支：
   - main: 稳定分支。
   - develop (开发分支): 最新的开发进度。所有的新工具都先合并到这里。
   - feat/<工具名>： 用来开发新的工具
   - fix/<工具名>/<修复名>：修复某个工具
2. 开发新工具
如果开发一个新的工具应该先起新的分支`feat/<工具名>`更新工具起点的版本号在新工具的第一次提交上。
需要时对一个新的工具进行适当拆分`feat/<工具名>/<功能名>`，依次使用不同的特性分支完成整体的开发。之后先合并入`feat/<工具名>`。
```bash
# 检查仓库状态
git status -sb
git checkout -b feat/<工具名> develop
# ... 写代码，提交 ...
git commit -m "feat: 中文描述...本次开发进度"
# 新的功能
git checkout -b feat/<工具名>/<功能名> feat/<工具名>
# ...继续写代码，提交...
git commit -m "feat: 中文描述...本次开发进度"
# 开发完成之后进入合成
```
3. 合并功能
```bash
# 功能合并到工具
git checkout feat/<工具名>
# 合并工具新特性
git merge --no-ff feat/<工具名>/<功能名>
# 删掉临时特性分支
git branch -d feat/<工具名>/<功能名>

# 确定改工具所有功能开发并测试完毕
# 最终合并
git checkout develop
git merge --no-ff feat/<工具名>
# 删掉临时的特性分支
git branch -d feat/<工具名>
# 继续开发新功能或者结束开发...
```
1. 发布版本：当develop上累积了足够的更新，合并到main，打标签，完成一个工具的开发。
```bash
# 1. 切换到 main 分支
git checkout main

# 2. 合并 develop，但强制生成合并节点 (--no-ff)，且暂停提交 (--no-commit)
# --no-ff: 即使可以快进，也强制生成一个 commit 节点，确保 main 上有一个独立的版本点
# --no-commit: 合并后暂不生成 commit，给你机会去删除不需要的文件
git merge --no-ff --no-commit develop

# 3. 排除不需要发布的文件夹/文件
# 将不需要的文件夹和文件从暂存区和工作区删除（仅在本次 main 分支的提交中删除，不影响 develop）
# 如果 main 上本来就不存在这些文件，而 develop 上有，这一步会阻止它们进入 main
git rm -r -f PLAN.md RULE.md 2>/dev/null || echo "文件夹不存在，跳过清理"

# 4. 提交合并，生成版本节点
git commit -m "tool: xxxxxx工具开发"

# 5. 推送
git push origin main --tags
```
5. 修复：如果发布后发现有严重 bug
从 main 切出 fix/<工具名>/<版本/特性分支> 分支进行修复
测试稳定后合并回develop和main