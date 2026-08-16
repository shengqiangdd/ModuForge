# 仓库规范（Repository Guidelines）

> 目的：保持仓库干净、可维护，**禁止上传不相关文件**。
> 所有新文件必须遵守本规范；`pre-commit` 钩子会自动拦截违规文件。

## 目录结构约定

| 路径 | 允许的内容 | 禁止的内容 |
|---|---|---|
| `backend/` | Go 后端源码、测试、Dockerfile | 编译出的二进制、本地数据 |
| `frontend/` | Svelte 前端源码 | `node_modules/`、`dist/` |
| `scripts/` | 构建/部署/检查脚本（build.sh、dev.sh、pre-commit-check.sh） | 一次性调试脚本 |
| `docs/` | 项目文档 | 过程记录、诊断报告 |
| `containers/` | builder 容器 Dockerfile | 构建产物 |
| 仓库根目录 | 项目级配置（docker-compose.yml、Makefile、README 等） | 散落脚本、任何杂项 |

## 禁止提交的文件类型

- **根目录散落的 `.py` / `.sh` / `.ps1` / `.bat`**：调试、诊断、一次性部署脚本一律禁止放在根目录
- **二进制与编译产物**：`*.exe`、`*.dll`、`*.so`、`*.o`、`*.bin`、编译出的 server 程序
- **日志与临时文件**：`*.log`、`*.tmp`、`*.swp`、`*.orig`、`*.rej`
- **备份与变体**：`*.bak`、`*.bak-*`、`*.hostmode`、`docker-compose.yml.bak*`
- **压缩包**：根目录 `*.zip`、`*.tar.gz`、`*.tgz`
- **敏感文件**：`.env`、含密钥/Token 的任何文件、测试脚本中硬编码的 API key
- **过程文档**：`OPTIMIZATION_*.md`、`DEPLOYMENT_SUCCESS.md`、`AGENT_DIAGNOSIS.md` 等一次性记录

## 提交前自检清单

1. `git status` 确认只暂存本次改动相关文件
2. 新增文件属于上述哪个允许目录？不在 `backend/ frontend/ scripts/ docs/ containers/` 就不提交
3. 内容是否包含密钥/Token？硬编码密钥一律禁止
4. 大文件（>1MB）？编译产物？——不提交

## 被拦截怎么办

`pre-commit` 钩子会拒绝提交并提示原因：

- 确属误拦截（必要文件）→ 调整 `.gitignore` 白名单或说明理由后 `git commit --no-verify`（应极少使用）
- 确属违规 → 删除该文件，按规范放入正确目录

## 例外

- `git add -f` 强制加入：仅限 `scripts/` 下确需入库但被 `scripts/` 忽略规则覆盖的脚本
- 任何 `--no-verify` / `-f` 使用，请在提交信息中说明原因
