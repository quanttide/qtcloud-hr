# CHANGELOG

## [0.1.11] - 2026-09-07

### Fixed

- 真实招聘动作现在必须通过 provider 环境就绪检查；简历预览和下载统一要求写权限，并记录审计日志。
- 收紧招聘审计目录权限，避免同机其他用户读取候选人操作记录。

## [0.1.10] - 2026-09-06

### Fixed

- 将 provider 环境检测的独立超时调整为 5 秒，避免 FC 冷启动或飞书 CLI 首次加载较慢时误报邮箱认证态不可用。

## [0.1.9] - 2026-09-06

### Fixed

- 限制 provider 环境检测的邮箱探测时长，避免 `lark-cli` 卡住时 API 网关返回 504；超时会返回脱敏的 `blocked` 状态。

## [0.1.8] - 2026-09-06

### Added

- 招聘 API 接入统一认证 `userinfo` 校验，按认证用户 `sub` 控制写权限。
- API 网关路由向 provider 注入共享校验头，阻断未经过网关的招聘 API 和简历临时链接直连。

## [0.1.7] - 2026-09-06

### Fixed

- 刷新持久化候选人快照后再执行查询和修改，降低 FC 多实例间使用旧快照覆盖新数据的风险。
- 持久化状态文件改为临时文件替换写入，并再次校验简历预览路径必须位于缓存目录内。

## [0.1.6] - 2026-09-06

### Fixed

- 将候选人快照与简历预览状态持久化到私有运行时目录，避免 FC 实例切换后出现 `candidate not found` 或临时预览失效。
- 为简历预览地址增加手动下载模式，PDF 预览保持 `inline`，下载时返回 `attachment`。

## [0.1.5] - 2026-09-06

### Fixed

- 移除已完成的一次性 Terraform state 迁移步骤，避免后续部署重复执行。
- FC 函数更新显式等待凭证 OSS RAM 策略绑定，并限制凭证挂载 endpoint 使用 HTTPS。

## [0.1.4] - 2026-09-06

### Fixed

- 修复 FC OSS 凭证挂载 endpoint 未使用 URL 格式导致部署失败的问题，并兼容已有的裸 endpoint 配置。
- 复用已预创建的 lark-cli 凭证 RAM 策略，避免部署 CI 身份缺少 `ram:ListTagResources` 时无法完成 Terraform Apply。

## [0.1.3] - 2026-09-06

### Added

- Provider 启动时显式设置 `HOME`、`LARKSUITE_CLI_CONFIG_DIR` 和 `LARKSUITE_CLI_DATA_DIR`，为 FC 运行时挂载生产 `lark-cli` 用户认证态提供稳定路径。
- Terraform 支持将私有 OSS 前缀挂载到 `/home/app` 承载 `lark-cli` 凭证目录，并给 FC 角色授予该前缀最小读写权限，以便 token 自动刷新。

### Fixed

- 修复 HR 邮箱认证态检测命令，改为使用当前 `lark-cli` 支持的邮箱文件夹只读探测，避免 `--mailbox` 参数不兼容导致误判失败。

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
