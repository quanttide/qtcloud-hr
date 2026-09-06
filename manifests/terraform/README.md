# qtcloud-human 部署选型（IaC）

对齐 qtdata、qtclass 与 qtrecurit 的部署模式，作为 Terraform 基础设施代码的设计依据。

## 部署选型

| 维度 | 选型 | 说明 |
|------|------|------|
| 客户端形态 | Flutter Web（量潮人事云工作台） | `src/studio`，`flutter build web --release` 产出站点 |
| 发布分发 | 阿里云 OSS 桶 `qtcloud-human-studio` | 静态网站托管（index.html 默认页）+ 公共读 |
| CDN | 阿里云 CDN `human.cloud.quanttide.com` | 源站 OSS（域名回源），单域名证书 `human.cloud.quanttide.com`（acme.sh dns_ali 签发，泛域名 `*.quanttide.com` 不匹配多级子域，续期后重跑 `scripts/configure-human-cdn.sh`） |
| 服务端 | 阿里云函数计算 FC 3.0 custom-container | `src/provider` 构建 `qtcloud-human-provider` 镜像，镜像内置 `human-server`、`qtrecurit`、`lark-cli` 和 `curl` |

## 本 IaC 范围

- **应用级**（`qtcloud-human-<env>` 命名）：FC 3.0 provider 函数、HTTP 触发器和 FC 默认 RAM 角色。
- **既有资源复用**：OSS 发布桶 `qtcloud-human-studio`、CDN `human.cloud.quanttide.com`、证书、ACR 仓库和 Terraform state bucket 不需要重建；CI/Terraform 只复用这些资源发布新版本。
- **不含** CDN / DNS / 证书的新建流程（无组织级 IaC 先例，在控制台配置并记录于本文件）

## studio 客户端发布

- 构建上传：`.github/workflows/deploy-studio.yml`（推送 tag `studio/*` 触发 → flutter build web → ossutil cp → 刷新 CDN）
- 必需变量：GitHub variable `QTCLOUD_HUMAN_API_BASE_URL` 必须指向 `https://api.quanttide.com/qtcloud-human` 网关地址，不得填写 FC HTTP 触发器直连地址。
- provider 部署必须配置 GitHub secret `QTCLOUD_HUMAN_GATEWAY_SHARED_SECRET`；Terraform 会拒绝空值，避免招聘 API 绕过网关直连。
- 统一认证用户可以各自登录；真实招聘写操作还需要把允许写入的认证用户 `sub` 放入 GitHub variable `QTCLOUD_HUMAN_RECRUITMENT_WRITERS`，逗号分隔。留空时所有用户只能读取，写操作返回 403。

## provider 服务端发布

- 构建部署：`.github/workflows/deploy-provider.yml`（推送 tag `provider/*` 触发 → Docker build/push → Terraform apply）。
- 镜像上下文必须从仓库根目录构建；`src/provider/Dockerfile` 会读取 human 仓库里的 `third_party/qtrecurit` submodule 并编译 `qtrecurit` CLI，不需要另建 `qtcloud-human-provider` OSS 桶。
- 最终镜像内置 `qtrecurit`、`lark-cli` 和 `curl`。其中 `qtrecurit` 负责招聘动作，`lark-cli` 负责读取/发送飞书邮箱，`curl` 负责下载简历附件。
- FC 环境变量默认设置 `HOME=/home/app`、`LARKSUITE_CLI_CONFIG_DIR=/home/app/.lark-cli`、`LARKSUITE_CLI_DATA_DIR=/home/app/.local/share`、`QTCLOUD_HUMAN_CACHE_HOME=/home/app/.qtcloud-human/cache`、`QTCLOUD_HUMAN_ACTION_LOG_DIR=/home/app/.qtcloud-human/audit`、`QTCLOUD_HUMAN_RECRUITMENT_STATE_PATH=/home/app/.qtcloud-human/state/recruitment-candidates.json`、`QTCLOUD_HUMAN_RESUME_VIEW_STATE_PATH=/home/app/.qtcloud-human/state/resume-views.json`、`QTCLOUD_HUMAN_CORS_ORIGINS=https://human.cloud.quanttide.com`、`QTRECURIT_DRY_RUN_DEFAULT=true`、`QTCLOUD_HUMAN_ALLOW_REAL_RECRUITMENT_ACTIONS=false`。
- 真实发送前需要确认 FC 运行环境里的 `lark-cli` 已具备访问 `hr@quanttide.com` 的认证上下文，并把 GitHub repository variable `QTCLOUD_HUMAN_ALLOW_REAL_RECRUITMENT_ACTIONS` 显式设为 `true`；否则页面只能 dry-run，真实收件箱同步、发送和简历下载会被 provider 拒绝。
- `lark-cli` 登录态必须在 provider 生产运行环境内建立或通过生产安全凭证介质挂载，不要提交到 Git。`config.json` 只是 CLI 配置索引，不能替代 token / app secret 所在的 CLI 凭证存储。Windows 本地凭证使用 DPAPI/注册表，不能直接复制到 Linux FC；生产凭证应在 Linux/FC 等价环境重新授权，或改造为服务端直接调用飞书 OpenAPI。
- 如通过 OSS 承载 provider 凭证目录，设置 GitHub repository variables `QTCLOUD_HUMAN_LARK_CLI_CREDENTIALS_OSS_BUCKET`、`QTCLOUD_HUMAN_LARK_CLI_CREDENTIALS_OSS_PREFIX`、可选的 `QTCLOUD_HUMAN_LARK_CLI_CREDENTIALS_OSS_ENDPOINT` 与可选的 `QTCLOUD_HUMAN_LARK_CLI_CREDENTIALS_OSS_POLICY_NAME` 后，部署 workflow 会传递 Terraform 变量 `lark_cli_credentials_oss_bucket`、`lark_cli_credentials_oss_prefix`、`lark_cli_credentials_oss_endpoint` 与 `lark_cli_credentials_oss_policy_name`。endpoint 可填写带或不带 `https://` 的地址，Terraform 会统一转换为 FC 要求的 HTTPS URL 格式。凭证 OSS 前缀所需的 RAM 自定义策略必须预先存在，默认名称为 `<project>-<environment>-lark-cli-credentials`，Terraform 只负责将该策略挂到 FC 角色；这样部署不依赖 CI 身份的 `ram:ListTagResources` 权限。Terraform 会把该前缀挂载到 `/home/app`，供 `lark-cli` 自动刷新用户 token。
- provider 的候选人快照、qtrecurit 邮件/简历缓存、预览 token 和动作日志也写入同一私有 OSS 挂载目录，避免 FC 实例切换后出现 `candidate not found` 或简历预览失效；该前缀必须保持私有，不得用于静态站点或公开下载。
- FC HTTP 触发器仍是匿名基础设施入口，但招聘路由已要求 API 网关注入的共享校验头；`X-Operator` 只保留给本地测试，不能作为身份认证。前端只能使用 API 网关地址，不能绕过网关访问 FC。
- 手动 workflow `.github/workflows/bootstrap-lark-cli-credentials.yml` 可在 GitHub Linux runner 上生成 provider 可用凭证目录并上传到私有 OSS 前缀；运行前需配置 repository secret `QTCLOUD_HUMAN_LARK_CLI_APP_SECRET` 和 repository variable `QTCLOUD_HUMAN_LARK_CLI_APP_ID`。

## 前后端发布顺序

1. 先运行系统级 API 网关部署脚本，确认 `/qtcloud-human/api/v1/recruitment/*` 路由已发布。
2. 设置 GitHub variable `QTCLOUD_HUMAN_API_BASE_URL=https://api.quanttide.com/qtcloud-human` 和 `QTCLOUD_HUMAN_AUTH_BASE_URL=https://api.quanttide.com/qtcloud-auth`。
3. 推送 `provider/*` tag，等待 provider 镜像构建和 Terraform Apply 成功。
4. 再推送 `studio/*` tag，构建时固化网关 API 地址和认证服务地址。
5. 发布后访问 `https://human.cloud.quanttide.com/`，先用 dry-run 验证候选人列表、报告、收件箱同步和动作按钮。

## 真实招聘动作启用门禁

1. 在受控环境完成 HR 邮箱用户授权，确保 `lark-cli` 对 `hr@quanttide.com` 有 `mail` 域读取和发送权限。
2. 在 Linux/FC 等价环境生成完整凭证目录：`/home/app/.lark-cli/config.json`、`/home/app/.local/share/lark-cli/master.key` 和对应 `*.enc` 凭证文件必须成组保留；不要从 Windows 本机复制 DPAPI 凭证。推荐手动运行 `Bootstrap Lark CLI Credentials` workflow，让 HR 用户通过 Step Summary 中的 URL/二维码授权。
3. 将该 `/home/app` 目录内容上传到私有 OSS 前缀，例如 `qtrecruit-private/lark-cli/hr-mailbox/home/`；该前缀不要用于静态站点或公开下载。
4. 确认 GitHub repository variables `QTCLOUD_HUMAN_LARK_CLI_CREDENTIALS_OSS_BUCKET` 和 `QTCLOUD_HUMAN_LARK_CLI_CREDENTIALS_OSS_PREFIX` 已配置，并确认默认 RAM policy `<project>-<environment>-lark-cli-credentials` 已限制到该 bucket/prefix；如使用其他策略名，额外配置 `QTCLOUD_HUMAN_LARK_CLI_CREDENTIALS_OSS_POLICY_NAME`。
5. 推送新的 `provider/*` tag 部署 provider，先保持 GitHub repository variable `QTCLOUD_HUMAN_ALLOW_REAL_RECRUITMENT_ACTIONS` 未设置或为 `false`。
6. 调用 `QTCLOUD_HUMAN_API_BASE_URL/api/v1/recruitment/provider/status`，确认 `qtrecurit` 与 `hr_mailbox` 两个组件均为 `ok`。
7. 将 GitHub repository variable `QTCLOUD_HUMAN_ALLOW_REAL_RECRUITMENT_ACTIONS` 设置为 `true`，再次推送新的 `provider/*` tag 触发部署。
8. 部署后先用小页数执行真实收件箱同步，再验证动作审计日志只包含元数据，不包含邮件正文、候选人邮箱、token 或环境变量。

## 关键操作记录（手动部署踩坑，源自 qtrecurit 经验）

1. **阻止公共访问**：2023 后新 OSS 桶默认开启"阻止公共访问"，即使 ACL=public-read 匿名访问也返回 `AccessDenied`。需用 `alicloud_oss_bucket_public_access_block` 独立资源显式关闭。
2. **ACL drift**：桶创建后 ACL 可能回退为 private，`terraform plan` 可检测并修复。
3. **CDN 配置**（控制台/CLI 完成，`scripts/configure-human-cdn.sh` 固化证书与 DNS）：
   - `AddCdnDomain`：`human.cloud.quanttide.com`，源站 OSS `qtcloud-human-studio.oss-cn-hangzhou.aliyuncs.com`（type=oss, port=443）
   - HTTPS：上传单域名证书（`SetCdnDomainSSLCertificate` CertType=upload；`acme.sh --issue --dns dns_ali -d human.cloud.quanttide.com` 签发，ZeroSSL 90 天，续期后重跑脚本）
   - DNS：`human.cloud.quanttide.com` CNAME → `human.cloud.quanttide.com.w.kunlunaq.com`（RR=`human.cloud`，注意精确匹配，避免被前缀记录误判）

## 使用

```sh
terraform init \
  -backend-config="bucket=quanttide-terraform-state" \
  -backend-config="key=qtcloud-human/terraform.tfstate" \
  -backend-config="region=cn-hangzhou"
terraform plan
terraform apply
```
