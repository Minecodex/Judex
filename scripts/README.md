# 工程脚本

- `npm run gen:api`：组装 V2 OpenAPI，生成按文件大小拆分的前端声明。
- `scripts/release/build.mjs VERSION`：固定源树、同版构建 Server／Web／CLI／Skill／Chart 与校验和，不发布外部服务。
- `scripts/release/images.mjs VERSION --load|--push`：校验同版制品与提交，构建本机验收镜像或推送三个 Linux AMD64／ARM64 Docker Hub 镜像，记录不可变索引摘要。详见[发行手册](../docs/运维/releases.md)。
- `scripts/release/client-downloads.mjs`：生成 Web 客户端下载包，版本通过 JUDEX_RELEASE_VERSION 传入。
- `scripts/deploy/load-local-image.mjs IMAGE ARCHIVE`：检查 Docker Desktop context 和真实节点架构，导入本机 Kubernetes 镜像。
- `scripts/operations`：外部数据库初始化、备份与恢复；操作前确认数据库与对象存储目标。
- `scripts/ci/pull-images.mjs`：CI 镜像拉取和有限重试。

正式自动化测试在 `tests/v2`，原型测试在 `tests/v2-preview`，说明见 [测试入口](../tests/README.md)。迁移账号工具为 `cmd/judex-account-import`，只导入认证记录，使用环境 DSN，不在参数或日志打印凭据。
