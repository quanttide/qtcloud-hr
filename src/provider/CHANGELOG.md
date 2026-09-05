# CHANGELOG

## [0.1.1] - 2026-09-05

### Fixed

- Provider 镜像内置 `qtrecurit` CLI，并补齐 `lark-cli` 与 `curl` 运行时依赖，支持招聘收件箱、报告、发送动作和简历附件下载。
- FC 运行时显式配置可写缓存目录、动作审计目录、CORS 来源和 qtrecurit dry-run 默认值，避免上线后缓存写入到不可控工作目录。
- 生产部署默认禁用真实招聘读信、发信和简历下载动作，必须显式开启 `QTCLOUD_HUMAN_ALLOW_REAL_RECRUITMENT_ACTIONS` 后才可执行。

### Docs

- 更新 Terraform 部署说明，明确复用既有 OSS、CDN、证书、ACR 和 Terraform state，并补充 provider/studio 前后端发布顺序。
