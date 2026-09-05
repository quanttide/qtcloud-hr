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
- 必需变量：GitHub variable `QTCLOUD_HUMAN_API_BASE_URL` 必须指向 provider API 公网地址，否则前端构建会失败，避免发布出无法调用后端的页面。

## provider 服务端发布

- 构建部署：`.github/workflows/deploy-provider.yml`（推送 tag `provider/*` 触发 → Docker build/push → Terraform apply）。
- 镜像上下文必须从仓库根目录构建；`src/provider/Dockerfile` 会读取 human 仓库里的 `third_party/qtrecurit` submodule 并编译 `qtrecurit` CLI，不需要另建 `qtcloud-human-provider` OSS 桶。
- 最终镜像内置 `qtrecurit`、`lark-cli` 和 `curl`。其中 `qtrecurit` 负责招聘动作，`lark-cli` 负责读取/发送飞书邮箱，`curl` 负责下载简历附件。
- FC 环境变量默认设置 `QTCLOUD_HUMAN_CACHE_HOME=/tmp/qtcloud-human/cache`、`QTCLOUD_HUMAN_ACTION_LOG_DIR=/tmp/qtcloud-human/audit`、`QTCLOUD_HUMAN_CORS_ORIGINS=https://human.cloud.quanttide.com`、`QTRECURIT_DRY_RUN_DEFAULT=true`、`QTCLOUD_HUMAN_ALLOW_REAL_RECRUITMENT_ACTIONS=false`。
- 真实发送前需要确认 FC 运行环境里的 `lark-cli` 已具备访问 `hr@quanttide.com` 的认证上下文，并把 `allow_real_recruitment_actions` 显式设为 `true`；否则页面只能 dry-run，真实收件箱同步、发送和简历下载会被 provider 拒绝。

## 前后端发布顺序

1. 先推送 `provider/*` tag，等待 provider 镜像构建和 Terraform Apply 成功。
2. 记录 `fc_http_url` 或后续 API 网关域名，写入 GitHub variable `QTCLOUD_HUMAN_API_BASE_URL`。
3. 再推送 `studio/*` tag，构建时通过 `--dart-define=QTCLOUD_HUMAN_API_BASE_URL=...` 固化前端 API 地址。
4. 发布后访问 `https://human.cloud.quanttide.com/`，用 dry-run 验证候选人列表、报告、收件箱同步和动作按钮。

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
