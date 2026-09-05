# 人事云工作台连接 qtrecurit CLI 能力设计

本文档说明如何把 `qtrecurit` CLI 已有的招聘能力迁移到量潮人事云工作台前端，重点覆盖当前确定需要接入的两类命令：`report` 和 `access`；招聘页面数据来源补充使用 `inbox sync` 拉取新投递候选人。实施计划见 [plan.md](./plan.md)。

目标不是让浏览器直接运行本地 CLI，而是把 CLI 背后的招聘动作沉淀为后端能力，再由前端按钮和 CLI 共同调用同一套服务接口。

## 背景

人事云工作台发布在 `https://human.cloud.quanttide.com/`，静态前端资源部署到阿里云 OSS 桶 `qtcloud-human-studio`，通过 CDN 域名 `human.cloud.quanttide.com` 对外访问。

当前招聘页面曾是前端 mock 形态：页面负责展示投递邮件、初筛状态和人工处理入口，但数据来源尚未接入正式后端。`qtrecurit` CLI 则已经沉淀了招聘邮箱读取、统计报告、问卷发送、实训邀约、笔试通知和面试通知等实际操作能力。

因此，人事云工作台的招聘页面应从“静态演示页面”升级为“招聘操作台”：前端提供按钮和状态展示，后端承接具体动作，CLI 和页面共用同一套招聘业务能力。

## 设计原则

1. 前端不直接执行 CLI：浏览器不能也不应该直接运行本地命令。
2. 后端不裸露 shell 能力：如果短期内需要调用 `qtrecurit`，必须通过受控 adapter 封装参数、超时、日志和错误处理。
3. CLI 能力服务化：`inbox sync`、`report` 和 `access` 的稳定业务语义应抽为后端 API。
4. 页面和 CLI 共用接口：前端按钮、后续新版 CLI、自动化任务都调用同一套招聘 API。
5. 发送类动作默认可审计：发送对象、模板、动作、结果和操作者必须记录，但不记录邮件正文等敏感内容。

## 当前 CLI 能力

### inbox sync

`qtrecurit inbox sync` 用于从 HR 收件箱拉取最近一批新投递，输出候选人最小字段。它是招聘页面替换本地 mock 数据的候选人来源入口，不承担简历解析、OCR、LLM 深度评分、全量历史扫描或候选人状态流转。

典型命令：

```bash
qtrecurit inbox sync --format json
qtrecurit inbox sync --mailbox hr@quanttide.com --folder INBOX --page-size 50 --format json
qtrecurit inbox sync --dry-run --format json
```

前端对应能力：

| CLI 能力 | 前端入口 | 页面输出 |
|----------|----------|----------|
| 拉取新投递 | “拉取新邮件”按钮 | 最近一批同步状态、扫描数、新增候选人数 |
| dry_run 同步 | dry_run 开关 + “拉取新邮件” | 预览同步语义，不读取邮箱、不写缓存 |

### report

`qtrecurit report` 用于生成招聘统计报告。它通过 `lark-cli` 读取 HR 邮箱邮件，按岗位规则分类，统计投递趋势和招聘漏斗，最终输出 Markdown 报告。

典型命令：

```bash
qtrecurit report
qtrecurit report --days 30
qtrecurit report --start 2026-06-01 --end 2026-06-30
```

前端对应能力：

| CLI 能力 | 前端入口 | 页面输出 |
|----------|----------|----------|
| 默认周期招聘报告 | “生成报告”按钮 | Markdown 预览、指标卡、趋势图 |
| 最近 N 天报告 | 日期快捷筛选 | 投递量、岗位分布、漏斗转化 |
| 指定日期范围报告 | 日期区间选择器 | 可复制/导出的招聘报告 |

### access

`qtrecurit access` 是候选人准入和考核流程动作集合。它最适合迁移为页面上的候选人操作按钮。

当前子命令：

```bash
qtrecurit access survey --to <候选人邮箱> --name <候选人姓名> [--link <问卷链接>] [--dry-run]
qtrecurit access invite --to <候选人邮箱> --name <候选人姓名> [--qr <二维码图片>] [--dry-run]
qtrecurit access exam --to <候选人邮箱> --name <候选人姓名> [--dry-run]
qtrecurit access interview --to <候选人邮箱> --name <候选人姓名> --position <岗位> --time <面试时间> [--dry-run]
```

前端对应按钮：

| CLI 命令 | 前端按钮 | 主要输入 | 结果 |
|----------|----------|----------|------|
| `access survey` | 发送问卷 | 候选人邮箱、姓名、问卷链接 | 发送问卷邮件，归档投递邮件 |
| `access invite` | 发送实训邀约 | 候选人邮箱、姓名、群二维码 | 发送实训基地邀请邮件 |
| `access exam` | 发送笔试 | 候选人邮箱、姓名 | 发送考核/笔试邀请，归档投递邮件 |
| `access interview` | 生成面试通知 | 候选人邮箱、姓名、岗位、时间 | 生成面试通知草稿 |

## 目标架构

推荐架构如下：

```text
人事云工作台前端
  -> qtcloud-human provider API
    -> qtrecurit 能力 adapter / 招聘领域服务
      -> Lark Mail / 模板 / 缓存 / 日志

qtrecurit CLI
  -> 同一套 provider API
```

短期可以让 provider 的 adapter 调用已安装的 `qtrecurit` CLI，但这只能作为过渡方案。长期应把 qtrecurit 的核心能力拆成可复用库或服务，CLI 只保留命令行入口。

## API 设计草案

### 生成招聘报告

```http
POST /api/v1/recruitment/reports
Content-Type: application/json
```

请求体：

```json
{
  "days": 30,
  "start": null,
  "end": null
}
```

响应体：

```json
{
  "report_id": "rpt_20260904_001",
  "markdown": "# 招聘统计报告\n...",
  "metrics": {
    "applications": 42,
    "exam": 12,
    "interview": 5,
    "offer": 1
  },
  "created_at": "2026-09-04T00:00:00Z"
}
```

前端行为：

1. 用户选择日期范围。
2. 点击“生成报告”。
3. 页面展示 loading 状态。
4. 后端返回 Markdown 和结构化指标。
5. 页面展示报告预览，并支持复制 Markdown。

### 同步收件箱新投递

```http
POST /api/v1/recruitment/inbox/sync
Content-Type: application/json
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

provider 只允许受控参数：`mailbox`、`folder`、`page_size`、`dry_run`，内部调用固定参数数组：`["inbox", "sync", "--mailbox", ..., "--folder", ..., "--format", "json"]`。审计日志只记录动作元数据、扫描数和导入数，不记录邮件正文、候选人邮箱、命令行、环境变量或 token。

`candidates` 返回当前扫描批次对应的候选人快照；`imported` 只表示本次新增导入数量。第一阶段不把本地缓存扩展为全量历史 ATS 列表，避免旧缓存或历史坏数据污染招聘页。

### 执行候选人动作

```http
POST /api/v1/recruitment/candidates/{candidate_id}/actions
Content-Type: application/json
```

发送问卷：

```json
{
  "action": "send_survey",
  "dry_run": false,
  "params": {
    "link": "https://example.com/survey"
  }
}
```

发送实训邀约：

```json
{
  "action": "send_training_invite",
  "dry_run": false,
  "params": {
    "qr": "oss://qtcloud-human-studio/assets/training-group.png"
  }
}
```

发送笔试：

```json
{
  "action": "send_exam",
  "dry_run": false,
  "params": {}
}
```

生成面试通知：

```json
{
  "action": "create_interview_notice",
  "dry_run": false,
  "params": {
    "position": "数据工程师",
    "time": "2026-09-10 10:00"
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
  "external_message_id": "om_xxx",
  "created_at": "2026-09-04T00:00:00Z"
}
```

候选人列表读取：

```http
GET /api/v1/recruitment/candidates
```

provider 第一阶段实现提供候选人列表骨架供招聘页读取。当前动作接口要求 `X-Operator` 和 `X-Recruitment-Permission: write|admin` 请求上下文，这只是开发期适配约定，生产部署必须接入真实认证和授权中间件，不能把普通客户端请求头当作最终权限来源。

状态建议：

| 状态 | 含义 |
|------|------|
| `draft` | 已生成草稿，待人工发送 |
| `sent` | 已发送 |
| `failed` | 执行动作失败 |
| `dry_run` | 仅预览，未发送 |

## 前端页面改造

招聘页面建议分为三块：

1. 收件箱列表：展示候选人姓名、邮箱、岗位、邮件摘要、当前阶段。
2. 候选人详情：展示投递正文、附件状态、初筛结论、历史动作。
3. 动作区：放置问卷、实训邀约、笔试、面试通知等按钮。

按钮行为建议：

| 按钮 | 触发条件 | 前端交互 |
|------|----------|----------|
| 拉取新邮件 | provider API 已配置 | loading、成功/失败提示；dry_run 不读取邮箱 |
| 发送问卷 | 候选人邮箱和姓名存在 | 二次确认，可选问卷链接 |
| 发送实训邀约 | 问卷通过或人工确认 | 二次确认，可选二维码附件 |
| 发送笔试 | 候选人进入考核阶段 | 二次确认 |
| 面试通知 | 候选人通过筛选/考核 | 弹窗填写岗位和面试时间 |
| 生成报告 | 任何时间 | 日期筛选后生成 Markdown |

所有发送类按钮都应有 loading、成功、失败和重试提示。失败信息只展示可操作原因，例如“邮箱缺失”“未配置问卷链接”“发送服务不可用”，不要把底层命令、token、堆栈暴露给用户。

## 后端 adapter 约束

如果第一阶段选择由 provider 调用 `qtrecurit` CLI，应设置以下边界：

1. 只允许白名单动作：`inbox sync`、`report`、`access survey`、`access invite`、`access exam`、`access interview`。
2. 参数必须结构化传入，不拼接 shell 字符串。
3. 每个动作设置超时，避免页面请求无限挂起。
4. 邮件正文、访问令牌、飞书凭证不得写入前端响应和业务日志。
5. `dry_run` 应保留，方便页面预览动作。
6. 发送结果必须写审计日志，至少包含候选人、动作、模板、状态、操作者和时间。
7. provider 以参数数组调用 CLI；`invite --qr` 的远程资源引用在服务端解析为本地附件前不得开放真实发送。

推荐 provider 内部接口：

```go
type RecruitmentActionRequest struct {
    Action string         `json:"action"`
    DryRun bool           `json:"dry_run"`
    Params map[string]any `json:"params"`
}

type RecruitmentActionResult struct {
    ActionID          string `json:"action_id"`
    CandidateID       string `json:"candidate_id"`
    Action            string `json:"action"`
    Status            string `json:"status"`
    Message           string `json:"message"`
    ExternalMessageID string `json:"external_message_id,omitempty"`
    CreatedAt         string `json:"created_at"`
}
```

## 数据模型建议

前端至少需要以下字段支撑动作按钮：

```json
{
  "id": "cand_001",
  "name": "张三",
  "email": "zhangsan@example.com",
  "subject": "应聘数据工程师",
  "body": "HR 您好，我想投递数据工程师岗位，附件是我的简历。",
  "position": "数据工程师",
  "stage": "new",
  "status": "pending",
  "has_resume": true,
  "has_cover_letter": false,
  "last_action": "send_survey",
  "updated_at": "2026-09-04T00:00:00Z"
}
```

`subject` 和 `body` 只用于招聘页面详情展示，帮助 HR 判断是否“仅简历”或包含正文/自荐内容；审计日志、错误响应和发送动作结果不得回传邮件正文、底层命令、token 或堆栈。

候选人阶段建议先保持简单：

| 阶段 | 含义 |
|------|------|
| `new` | 新投递 |
| `survey_sent` | 已发送问卷 |
| `invite_sent` | 已发送实训邀约 |
| `exam_sent` | 已发送笔试 |
| `interview_draft` | 已生成面试通知 |
| `archived` | 已归档 |

`pass` / `reject` 暂不属于当前 CLI 已有能力。后续若需要，应新增候选人状态流转 API，而不是硬塞进 `access` 命令。

## 实施步骤

第一阶段：最小闭环。

1. 在 provider 增加收件箱同步 API：对齐 `qtrecurit inbox sync --format json`。
2. 在 provider 增加招聘报告 API：对齐 `qtrecurit report`。
3. 在 provider 增加候选人动作 API：先支持 `send_survey`、`send_training_invite`、`send_exam`、`create_interview_notice`。
4. 前端招聘页从 mock 数据切换为 API 数据，并提供“拉取新邮件”按钮。
5. 在候选人详情区增加四个动作按钮。
6. 所有发送动作先支持 `dry_run`，通过后再开放真实发送。

第二阶段：CLI 与页面统一。

1. 让新版 `qtrecurit` CLI 支持通过 `QTCLOUD_HUMAN_API_BASE_URL` 调用 provider。
2. 保留原本本地/飞书直连模式作为开发 fallback。
3. 页面和 CLI 的动作结果统一写入后端审计日志。

第三阶段：领域能力抽象。

1. 将 `qtrecurit` 邮件、模板、缓存、归档逻辑抽成可复用库或服务。
2. provider 直接调用库/服务，不再调用 CLI 进程。
3. CLI 只作为命令行入口，减少重复业务逻辑。

## 验收标准

1. 招聘页面能通过 provider 拉取新投递候选人，不再依赖招聘页本地 mock。
2. 招聘页面能生成与 `qtrecurit report` 等价的报告内容。
3. 招聘页面能对候选人执行发送问卷、发送实训邀约、发送笔试、生成面试通知四类动作。
4. 每个按钮执行后页面能展示动作状态，并记录到候选人动作历史。
5. 后端不会向前端返回底层命令、密钥、token 或邮件正文。
6. 所有发送类动作支持 `dry_run` 验证。
7. `qtrecurit` CLI 和前端页面最终能通过同一套 provider API 操作同一批招聘数据。
