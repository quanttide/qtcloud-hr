# CHANGELOG

## [0.1.0-alpha.5] - 2026-09-06

### Changed

- 取消候选人切换和页面加载时的简历自动下载。
- 简历附件改为明确的“预览”和“下载”手动操作，PDF 在页面内嵌预览。
- 候选人快照过期时提示重新拉取新邮件，避免直接显示英文 `candidate not found`。

## [0.1.0-alpha.4] - 2026-09-06

### Added

- 招聘页新增“检测环境”入口，可从网页检查 provider 容器内 `qtrecurit` CLI 与 HR 邮箱认证态是否就绪。
- 新增 provider 状态提示条，向使用者展示可执行/不可用原因，避免同事上线后误以为浏览器会读取本地 CLI。

## [0.1.0-alpha.3] - 2026-09-05

### Fixed

- Studio 发布构建注入 `QTCLOUD_HUMAN_API_BASE_URL`，避免上线后招聘页仍处于 provider API 未配置状态。
- Studio 发布流水线在缺少 provider API 地址时直接失败，防止静态页面和后端发布脱节。
