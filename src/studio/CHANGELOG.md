# CHANGELOG

## [0.1.0-alpha.3] - 2026-09-05

### Fixed

- Studio 发布构建注入 `QTCLOUD_HUMAN_API_BASE_URL`，避免上线后招聘页仍处于 provider API 未配置状态。
- Studio 发布流水线在缺少 provider API 地址时直接失败，防止静态页面和后端发布脱节。
