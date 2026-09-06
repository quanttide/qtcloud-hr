# CHANGELOG

## [0.1.2] - 2026-09-06

### Added

- 新增招聘 provider 运行环境检测接口 `/api/v1/recruitment/provider/status`，用于确认 provider 容器内 `qtrecurit` CLI 和 HR 邮箱认证态是否可用。
- 检测结果只返回脱敏的可用性、邮箱地址和组件状态，不暴露 token、认证详情、邮件正文或底层错误输出。

## [0.1.1] - 2026-09-05

### Fixed

- Provider 镜像内置 `qtrecurit` CLI，并补齐 `lark-cli` 与 `curl` 运行时依赖，支持招聘收件箱、报告、发送动作和简历附件下载。
- FC 运行时显式配置可写缓存目录、动作审计目录、CORS 来源和 qtrecurit dry-run 默认值，避免上线后缓存写入到不可控工作目录。
- 生产部署默认禁用真实招聘读信、发信和简历下载动作，必须显式开启 `QTCLOUD_HUMAN_ALLOW_REAL_RECRUITMENT_ACTIONS` 后才可执行。

### Docs

- 更新 Terraform 部署说明，明确复用既有 OSS、CDN、证书、ACR 和 Terraform state，并补充 provider/studio 前后端发布顺序。
