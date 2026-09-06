variable "region" {
  description = "阿里云地域"
  type        = string
  default     = "cn-hangzhou"
}

variable "project" {
  description = "项目名（资源命名前缀）"
  type        = string
  default     = "qtcloud-human"
}

variable "environment" {
  description = "环境：dev / prod"
  type        = string
  default     = "prod"
}

variable "image" {
  description = "FC 容器镜像（ACR 地址）。由 CI 注入"
  type        = string
}

variable "fc_memory" {
  description = "FC 函数内存（MB）"
  type        = number
  default     = 512
}

variable "fc_timeout" {
  description = "FC 函数超时（秒）"
  type        = number
  default     = 60
}

variable "cors_origins" {
  description = "允许访问 provider API 的前端来源，逗号分隔"
  type        = string
  default     = "https://human.cloud.quanttide.com"
}

variable "qtrecurit_dry_run_default" {
  description = "招聘 API 未显式传 dry_run 时是否默认预览、不真实发送"
  type        = bool
  default     = true
}

variable "allow_real_recruitment_actions" {
  description = "是否允许 provider 执行真实招聘读信、发信和简历下载动作"
  type        = bool
  default     = false
}

variable "qtrecurit_timeout_seconds" {
  description = "provider 调用 qtrecurit CLI 的超时时间（秒）"
  type        = number
  default     = 60
}

variable "lark_cli_credentials_oss_bucket" {
  description = "可选：保存 provider 生产 lark-cli 用户认证态的私有 OSS bucket。留空则不挂载"
  type        = string
  default     = ""
}

variable "lark_cli_credentials_oss_prefix" {
  description = "可选：OSS bucket 内的 lark-cli 凭证目录前缀，应包含 .lark-cli 与 .local/share/lark-cli 内容"
  type        = string
  default     = ""
}

variable "lark_cli_credentials_oss_endpoint" {
  description = "可选：lark-cli 凭证 OSS mount endpoint"
  type        = string
  default     = "oss-cn-hangzhou-internal.aliyuncs.com"
}
