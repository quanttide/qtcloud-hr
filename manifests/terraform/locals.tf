locals {
  # 应用级资源命名：<app>-<env>（系统级资源由 quanttide-platform 管理）
  app_name_prefix                     = "${var.project}-${var.environment}"
  lark_cli_credentials_mount_enabled  = trimspace(var.lark_cli_credentials_oss_bucket) != "" && trimspace(var.lark_cli_credentials_oss_prefix) != ""
  lark_cli_credentials_oss_key_prefix = trimsuffix(trimprefix(trimspace(var.lark_cli_credentials_oss_prefix), "/"), "/")
  lark_cli_credentials_bucket_path    = "/${local.lark_cli_credentials_oss_key_prefix}"
}
