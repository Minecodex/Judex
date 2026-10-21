# 公开发行

公开版本使用 SemVer；例如制品版本 `0.0.1-beta` 对应 Git 标签和镜像标签 `v0.0.1-beta`。内部 V2 阶段编号与公开发行版本分开。首次发行说明见 [v0.0.1-beta](../releases/v0.0.1-beta.md)。

## 准备与构建

1. 同步 README、Chart、npm workspace、Skill 版本与 `values-beta.yaml` 的三个镜像标签。
2. 完成 Go／Web／部署验证并提交；公开制品应从干净、已提交的源码构建。
3. 构建并验证固定来源的发行包：

```sh
JUDEX_PYTHON=python3 node scripts/release/build.mjs 0.0.1-beta
JUDEX_PYTHON=python3 node tests/release.mjs dist/release/0.0.1-beta
```

构建器记录完整提交、源码摘要及三个镜像的目标平台，将 Chart、配置示例、说明、CLI、Server、Web、Skill、OpenAPI 和许可证写入 `dist/release/VERSION`。`checksums.txt` 仅列公开的顶层文件；内部展开目录不作为 Release 附件。

## 容器与 Kubernetes 验收

```sh
# 构建当前机器可加载的镜像，供隔离验收使用
node scripts/release/images.mjs 0.0.1-beta --load
```

Server Dockerfile 使用发行包中的两架构二进制和 Web，不再次编译。镜像脚本校验制品校验和、完整提交和源码摘要；转换器与分析沙箱从同一源码构建。`--load` 只构建宿主架构；正式 `--push` 构建 `linux/amd64,linux/arm64`。

按 [测试说明](../../tests/README.md)将同版镜像导入临时 Kubernetes namespace，运行完整流程并核对 finally 清理。已有共享 Controller 的集群复用现有 Controller，不覆盖无关资源。先记录实际通过范围，再进行公开发布。

## 推送与发布

本机需要有目标 Docker Hub 账户的登录凭据，以及可发布 GitHub Release 的 `gh` 认证。凭据不写入仓库或构建上下文。

```sh
node scripts/release/images.mjs 0.0.1-beta --push
```

推送后输出 `dist/publish/0.0.1-beta/images.json` 及其独立 SHA-256，记录三个多架构索引摘要。以匿名 Registry 请求验证每个索引均公开可读且含 AMD64／ARM64，再创建不可变 Git 标签并推送相同提交。

将 `dist/release/VERSION` 的顶层文件（含 `checksums.txt`）、`images.json` 和 `images.json.sha256` 上传 GitHub Release，使用 [版本说明](../releases/v0.0.1-beta.md)作为正文，Beta 标记为 prerelease。发布前再次验证 Chart 中的镜像引用与 Docker Hub 摘要；公开后检查附件下载与 README 的安装链接。

`.github/workflows/release.yml` 是标签触发的完整候选验收，包括源码 CI、六平台原生 CLI 和两架构 Kubernetes 回归。公开推送由上述显式命令完成；工作流不保存 Docker Hub 凭据。只有实际完成的执行可以记录为通过，托管 CI 的状态另行核对。
