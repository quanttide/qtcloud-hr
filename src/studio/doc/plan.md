# 人事云工作台接入 qtrecurit 能力实施计划

本文档定义人事云工作台接入 `qtrecurit` 招聘能力的实施范围、边界、步骤和验收标准。详细架构设计见 [qtrecurit-cli-integration.md](./qtrecurit-cli-integration.md)。

## 目标

把 `qtrecurit` CLI 中已经验证过的 `report` 和 `access` 能力，转化为人事云工作台招聘页面可使用的后端 API 和前端按钮。

第一阶段只做两个能力域：

1. `report`：生成招聘统计报告。
2. `access`：执行候选人准入和考核流程沟通动作。

招聘页面候选人来源补充接入 `inbox sync`：它只用于从 HR 收件箱拉取最近一批新投递并产出候选人最小字段，作为替换页面 mock 数据的数据入口；不扩展为完整 ATS、全量历史扫描、简历解析或状态流转能力。

最终形态是：人事云工作台前端、`qtrecurit` CLI 和后续自动化任务共用同一套招聘服务能力。

## 当前状态

### qtcloud-human

人事云工作台当前发布到 `https://human.cloud.quanttide.com/`，静态资源部署在阿里云 OSS 桶 `qtcloud-human-studio`。

当前限制：

1. 招聘页面仍是 mock 数据驱动。
2. 正式 `src/provider` 尚未承接招聘 API。
3. `examples/human-api` 中已有员工、部门、岗位、简历导入、面试创建的参考 API，但还不是正式服务入口。

### qtrecurit

`qtrecurit` CLI 已有以下可迁移能力：

| 能力域 | 命令 | 用途 |
|--------|------|------|
| `inbox` | `qtrecurit inbox sync --format json` | 拉取 HR 收件箱新投递，输出候选人最小字段 |
| `report` | `qtrecurit report` | 读取 HR 邮箱，生成招聘统计报告 |
| `access` | `qtrecurit access survey` | 发送准入问卷 |
| `access` | `qtrecurit access invite` | 发送实训邀约 |
| `access` | `qtrecurit access exam` | 发送笔试/考核邀请 |
| `access` | `qtrecurit access interview` | 生成面试通知草稿 |

当前限制：

1. CLI 主要通过 `lark-cli` 连接 HR 邮箱。
2. 部分状态依赖本地缓存，例如问卷链接、邮箱文件夹 ID、二维码路径。
3. CLI 还没有候选人 `pass` / `reject` 的状态流转命令。

## 范围

### 本期范围

本期只纳入以下功能：

1. 前端招聘页通过 provider 拉取新投递候选人，替换本地 mock 数据。
2. 前端招聘页调用后端生成招聘报告。
3. 前端候选人详情页增加四个动作按钮：发送问卷、发送实训邀约、发送笔试、生成面试通知。
4. 后端提供 Action API，把页面动作映射到 `qtrecurit inbox sync`、`qtrecurit report` 和 `qtrecurit access` 的业务语义。
5. 后端记录动作审计日志。
6. 支持 `dry_run`，用于页面预览和联调验证。

### 不在本期范围

以下内容不在本期实现：

1. 浏览器直接执行本地 CLI。
2. 页面直接连接 `lark-cli` 或飞书凭证。
3. 完整候选人 ATS 系统。
4. `pass` / `reject` 状态流转。
5. 自动排期、日历邀约和会议室预定。
6. 简历附件解析、OCR、LLM 深度评分。
7. 多租户权限模型。
8. 替换 `qtrecurit` CLI 现有命令结构。

`pass` / `reject` 后续应作为候选人状态动作单独设计，不混入 `access` 命令。

## 边界

### 前端边界

前端只负责：

1. 展示候选人列表、详情、报告和动作历史。
2. 收集动作所需的少量参数。
3. 调用 provider API。
4. 展示动作执行状态、错误提示和 `dry_run` 预览结果。

前端不负责：

1. 执行 CLI 命令。
2. 保存飞书、邮箱或 OSS 密钥。
3. 拼接邮件正文。
4. 判断底层发送是否成功。
5. 操作邮箱文件夹。

### 后端边界

后端负责：

1. 暴露招聘报告 API 和候选人动作 API。
2. 做输入校验、权限校验、动作白名单校验。
3. 调用 `qtrecurit` 能力 adapter。
4. 标准化返回结果。
5. 记录审计日志。

后端不应该：

1. 透传任意 CLI 参数。
2. 拼接 shell 字符串执行命令。
3. 把底层命令、堆栈、token、邮件正文返回给前端。
4. 在同步 HTTP 请求里执行无超时的长任务。

### qtrecurit 边界

短期内 `qtrecurit` 可以作为能力来源：

1. `report` 输出 Markdown 报告。
2. `access` 子命令完成发送或草稿创建。
3. 继续复用现有模板、缓存、邮箱归档能力。

长期应逐步把核心逻辑从 CLI 入口下沉为库或服务：

1. CLI 调用库或 provider API。
2. provider 调用同一套库或服务。
3. 避免 provider 长期通过子进程调用 CLI。

## 功能映射

| 页面功能 | 后端动作 | qtrecurit 对应命令 | 备注 |
|----------|----------|--------------------|------|
| 拉取新邮件 | `sync_inbox` | `qtrecurit inbox sync --format json` | 只导入候选人最小字段，替换页面 mock 数据 |
| 生成招聘报告 | `create_report` | `qtrecurit report` | 支持默认周期、最近 N 天、日期区间 |
| 发送问卷 | `send_survey` | `qtrecurit access survey` | 可传问卷链接；未传时后端按缓存策略获取 |
| 发送实训邀约 | `send_training_invite` | `qtrecurit access invite` | 可传二维码资源 |
| 发送笔试 | `send_exam` | `qtrecurit access exam` | 发送后归档投递邮件 |
| 生成面试通知 | `create_interview_notice` | `qtrecurit access interview` | 需要岗位和面试时间 |

## API 计划

### 招聘报告

```http
POST /api/v1/recruitment/reports
```

请求体：

```json
{
  "days": 30,
  "start": null,
  "end": null,
  "dry_run": false
}
```

响应体：

```json
{
  "report_id": "rpt_20260904_001",
  "status": "created",
  "markdown": "# 招聘统计报告\n...",
  "created_at": "2026-09-04T00:00:00Z"
}
```

### 候选人动作

```http
POST /api/v1/recruitment/candidates/{candidate_id}/actions
```

请求体：

```json
{
  "action": "send_survey",
  "dry_run": false,
  "params": {
    "link": "https://example.com/survey"
  }
}
```

响应体：

```json
{
  "action_id": "act_20260904_001",
  "candidate_id": "cand_001",
  "action": "send_survey",
  "status": "sent",
  "message": "问卷邮件已发送",
  "created_at": "2026-09-04T00:00:00Z"
}
```

### 候选人列表

```http
GET /api/v1/recruitment/candidates
```

### 收件箱同步

```http
POST /api/v1/recruitment/inbox/sync
```

请求体：

```json
{
  "mailbox": "hr@quanttide.com",
  "folder": "INBOX",
  "page_size": 50,
  "dry_run": true
}
```

响应体：

```json
{
  "sync_id": "sync_20260904_001",
  "status": "synced",
  "mailbox": "hr@quanttide.com",
  "folder": "INBOX",
  "scanned": 50,
  "imported": 3,
  "candidates": [],
  "created_at": "2026-09-04T00:00:00Z"
}
```

`sync_inbox` 与发送动作一样需要服务端注入操作者身份和招聘写权限；日志只记录同步动作、操作者、扫描数和导入数，不记录邮件正文、候选人邮箱或 token。

用于招聘页面读取候选人最小字段。列表和动作接口都要求服务端注入已认证的操作者身份；当前 provider 开发实现使用 `X-Operator` 和 `X-Recruitment-Permission` 作为临时请求上下文，生产环境必须由网关或认证中间件替换为不可伪造的身份与权限信息。

## 数据要求

候选人记录至少需要：

| 字段 | 必填 | 说明 |
|------|------|------|
| `id` | 是 | 候选人唯一 ID |
| `name` | 是 | 候选人姓名 |
| `email` | 是 | 候选人邮箱，用于发送邮件 |
| `subject` | 否 | 原始投递邮件主题，用于页面详情展示 |
| `body` | 否 | 原始投递邮件正文纯文本，用于页面详情展示；不得写入审计日志或错误响应 |
| `position` | 否 | 应聘岗位，面试通知必填 |
| `stage` | 是 | 当前招聘阶段 |
| `status` | 是 | 当前处理状态 |
| `source_message_id` | 否 | 原始投递邮件 ID，用于归档和追踪 |
| `last_action` | 否 | 最近一次动作 |
| `updated_at` | 是 | 更新时间 |

动作日志至少需要：

| 字段 | 说明 |
|------|------|
| `action_id` | 动作 ID |
| `candidate_id` | 候选人 ID |
| `action` | 动作类型 |
| `operator` | 操作者 |
| `status` | `draft` / `sent` / `failed` / `dry_run` |
| `message` | 用户可读结果 |
| `external_message_id` | 邮件系统消息 ID，可为空 |
| `created_at` | 创建时间 |

日志不记录邮件正文、token、密钥、候选人邮箱、完整命令行参数。

## 安全要求

1. 所有动作必须鉴权后调用。
2. 发送类动作必须校验操作者权限。
3. 动作名称必须使用白名单，不接受任意字符串映射命令。
4. 邮箱、姓名、岗位、时间、链接等输入必须做格式和长度校验。
5. provider 调 CLI 时必须使用参数数组，不使用 shell 字符串。
6. 子进程必须设置超时。
7. 错误响应不得暴露底层命令、环境变量、token、堆栈。
8. 真实发送前应支持 `dry_run` 或二次确认。
9. 发送日志只记录元数据。
10. 前端不得持有飞书、邮箱、OSS 等服务端凭证。

## 实施步骤

### 第 0 步：确认接口契约

产出：冻结第一版 API 契约和字段定义。

任务：

1. 确认 `report` 和 `access` 是第一版唯一范围。
2. 确认候选人最小字段。
3. 确认四个前端按钮文案和参数。
4. 确认发送动作默认行为：真实发送或先 `dry_run`。

验收：

1. API 文档包含报告接口和动作接口。
2. 页面按钮与 qtrecurit 命令存在一一映射。
3. `pass` / `reject` 明确暂缓。

### 第 1 步：provider 增加招聘动作骨架

产出：`src/provider` 中可测试的招聘 API handler。

任务：

1. 新增招聘报告 handler。
2. 新增候选人动作 handler。
3. 新增请求/响应模型。
4. 新增动作白名单和参数校验。
5. 新增单元测试和集成测试。

验收：

1. `POST /api/v1/recruitment/reports` 能返回 mock 报告。
2. `POST /api/v1/recruitment/candidates/{id}/actions` 能处理四种动作。
3. 非法动作返回 400。
4. 缺少必填参数返回 400。
5. 测试通过。

### 第 2 步：接入 qtrecurit adapter

产出：provider 能通过受控 adapter 调用 `qtrecurit` 能力。

任务：

1. 封装 `qtrecurit report` 调用。
2. 封装 `qtrecurit access survey` 调用。
3. 封装 `qtrecurit access invite` 调用。
4. 封装 `qtrecurit access exam` 调用。
5. 封装 `qtrecurit access interview` 调用。
6. 设置超时、错误归一化和审计日志。

验收：

1. adapter 不拼接 shell 字符串。
2. 每个动作支持 `dry_run`。
3. 执行失败时返回可读错误。
4. 不向前端暴露底层命令和敏感信息。
5. 动作日志写入成功。

### 第 3 步：前端招聘页 API 化

产出：招聘页面从 mock 数据切换到 provider API。

任务：

1. 抽出招聘 API client。
2. 候选人列表从 API 读取。
3. 候选人详情展示 API 数据。
4. 报告区域调用报告 API。
5. 增加“拉取新邮件”按钮调用 `sync_inbox`，用 provider 候选人列表替换本地 mock。

验收：

1. 页面可加载候选人列表。
2. 无后端时能清晰展示错误。
3. 生成报告按钮能展示 Markdown 报告。
4. 页面 loading、empty、error 状态完整。

### 第 4 步：前端动作按钮接入

产出：招聘页面具备 `access` 四个动作按钮。

任务：

1. 增加“发送问卷”按钮。
2. 增加“发送实训邀约”按钮。
3. 增加“发送笔试”按钮。
4. 增加“生成面试通知”按钮和面试时间输入。
5. 增加动作执行结果提示和动作历史展示。

验收：

1. 每个按钮能调用对应后端动作。
2. 执行中按钮不可重复点击。
3. 成功后刷新候选人状态或动作历史。
4. 失败时展示用户可处理的错误提示。
5. `dry_run` 模式下不真实发送。

### 第 5 步：联调和发布准备

产出：可部署的人事云工作台招聘操作闭环。

任务：

1. 本地联调前端和 provider。
2. 使用 `dry_run` 验证四类动作。
3. 使用测试候选人验证真实发送流程。
4. 检查日志和错误处理。
5. 更新部署说明和变更记录。

验收：

1. 前端构建通过。
2. provider 测试通过。
3. `dry_run` 不产生真实邮件。
4. 真实发送只对测试对象执行。
5. 部署文档说明 API 地址、环境变量和回滚方式。

## 环境变量计划

provider 侧建议：

| 变量 | 用途 |
|------|------|
| `QTRECURIT_BIN` | `qtrecurit` 可执行文件路径，默认 `qtrecurit` |
| `QTRECURIT_TIMEOUT_SECONDS` | CLI 调用超时时间 |
| `QTRECURIT_DRY_RUN_DEFAULT` | 是否默认 dry-run |
| `QTCLOUD_HUMAN_ACTION_LOG_DIR` | 动作日志目录 |
| `QTCLOUD_HUMAN_CORS_ORIGINS` | 本地/部署前端允许跨域来源，逗号分隔 |
| `LARK_CLI_BIN` | `lark-cli` 可执行文件路径 |

provider 当前使用内存候选人存储作为第一阶段骨架，正式环境需要替换为持久化候选人/邮件数据源。`access invite` 的 `qr` 参数当前只做受控资源引用校验；qtrecurit CLI 仍要求本地附件路径，正式发送前需要增加服务端资源解析器。

前端侧建议：

| 变量 | 用途 |
|------|------|
| `QTCLOUD_HUMAN_API_BASE_URL` | provider API 地址 |

生产环境凭证只保存在服务端运行环境，不能进入前端构建产物。

## 风险和处理

| 风险 | 影响 | 处理 |
|------|------|------|
| provider 直接调 CLI 稳定性不足 | 请求超时或失败 | 设置超时、错误归一化、后续抽库 |
| 邮件真实误发 | 影响候选人体验 | 默认 `dry_run`、二次确认、测试白名单 |
| 前端暴露敏感信息 | 安全事故 | 前端不接触凭证，错误脱敏 |
| qtrecurit 本地缓存依赖强 | 部署环境不可复现 | 缓存路径显式配置，关键配置迁入 provider |
| report 生成耗时 | 页面等待过久 | 第一版同步返回，后续改异步任务 |
| pass/reject 被误认为已支持 | 产品预期偏差 | 文档明确本期不含，后续另设状态流转 API |

## 完成定义

本计划完成时应满足：

1. 招聘页面能生成 `qtrecurit report` 等价的报告。
2. 招聘页面能执行发送问卷、发送实训邀约、发送笔试、生成面试通知。
3. 页面动作和 CLI 动作语义一致。
4. provider 有输入校验、动作白名单、超时和审计日志。
5. 前端不直接运行 CLI，不保存服务端凭证。
6. `dry_run`、测试、文档和部署说明齐备。
