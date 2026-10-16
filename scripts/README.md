# 开发与运维脚本

从仓库根目录执行。应用运行入口仍位于 `cmd/`，以下工具用于开发生成、发行和运维。

| 目录 | 用途 | 调用入口 |
| --- | --- | --- |
| [split-api](split-api/main.go) | 按 Go 声明拆分生成类型，补齐各分片导入并检查每个文件不超过 2000 行 | [API 类型生成](../api/generate-types.mjs)，通过 `npm run gen:api` 或 `make generate` 调用 |
| [release](release/build.mjs) | 构建发行制品、客户端／Skill 下载包和校验清单 | `npm run release:build`；前端 postbuild 使用 `release/client-downloads.mjs` |
| [deploy](deploy/load-local-image.mjs) | 将本地 Docker 镜像导入 Kubernetes 节点镜像存储 | 见[部署说明](../deploy/README.md) |
| [operations](operations/backup.sh) | PostgreSQL 初始化、完整备份及隔离恢复 | 见[运维手册](../docs/运维/operations.md) |

OpenAPI 作者文件和契约生成入口继续位于 `api/`；`split-api` 是其 Go 辅助工具。生成后的后端类型位于 `internal/gen/api/`，前端类型位于 `web/src/lib/api/`。
