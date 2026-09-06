# FC 默认角色：允许 FC 服务挂载弹性网卡访问 VPC（应用级）
resource "alicloud_ram_role" "fc" {
  role_name = "${local.app_name_prefix}-fc"
  assume_role_policy_document = jsonencode({
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = ["fc.aliyuncs.com"] }
    }]
    Version = "1"
  })
  description = "Function Compute 默认角色（qtcloud-human）"
}

resource "alicloud_ram_role_policy_attachment" "fc_vpc" {
  policy_name = "AliyunECSNetworkInterfaceManagementAccess"
  policy_type = "System"
  role_name   = alicloud_ram_role.fc.role_name
}

resource "alicloud_ram_role_policy_attachment" "fc_lark_cli_credentials" {
  count       = local.lark_cli_credentials_mount_enabled ? 1 : 0
  policy_name = local.lark_cli_credentials_policy_name
  policy_type = "Custom"
  role_name   = alicloud_ram_role.fc.role_name
}

# 函数计算（FC 3.0）：custom-container 容器镜像，内置 qtrecurit CLI
resource "alicloud_fcv3_function" "this" {
  depends_on = [
    alicloud_ram_role_policy_attachment.fc_vpc,
    alicloud_ram_role_policy_attachment.fc_lark_cli_credentials,
  ]

  function_name   = local.app_name_prefix
  description     = "qtcloud-human 人力资源 API"
  runtime         = "custom-container"
  handler         = "index.handler"
  cpu             = 0.5
  memory_size     = var.fc_memory
  disk_size       = 512
  timeout         = var.fc_timeout
  internet_access = true
  role            = alicloud_ram_role.fc.arn

  custom_container_config {
    image = var.image
    port  = 8080
  }

  # 对齐 provider 运行时约定：容器监听 8080，招聘 API 默认 dry-run，
  # qtrecurit 收件箱/简历缓存和动作审计日志写入 FC 可写临时目录。
  environment_variables = {
    HOME                                         = "/home/app"
    LISTEN_ADDR                                  = ":8080"
    LARKSUITE_CLI_CONFIG_DIR                     = "/home/app/.lark-cli"
    LARKSUITE_CLI_DATA_DIR                       = "/home/app/.local/share"
    LARKSUITE_CLI_NO_SKILLS_NOTIFIER             = "1"
    LARKSUITE_CLI_NO_UPDATE_NOTIFIER             = "1"
    QTCLOUD_HUMAN_CACHE_HOME                     = "/home/app/.qtcloud-human/cache"
    QTCLOUD_HUMAN_ACTION_LOG_DIR                 = "/home/app/.qtcloud-human/audit"
    QTCLOUD_HUMAN_RECRUITMENT_STATE_PATH         = "/home/app/.qtcloud-human/state/recruitment-candidates.json"
    QTCLOUD_HUMAN_RESUME_VIEW_STATE_PATH         = "/home/app/.qtcloud-human/state/resume-views.json"
    QTCLOUD_HUMAN_ALLOW_REAL_RECRUITMENT_ACTIONS = tostring(var.allow_real_recruitment_actions)
    QTCLOUD_HUMAN_AUTH_USERINFO_URL             = var.auth_userinfo_url
    QTCLOUD_HUMAN_RECRUITMENT_WRITERS           = var.recruitment_writers
    QTCLOUD_HUMAN_GATEWAY_SHARED_SECRET         = var.gateway_shared_secret
    QTCLOUD_HUMAN_CORS_ORIGINS                   = var.cors_origins
    QTRECURIT_BIN                                = "/usr/local/bin/qtrecurit"
    QTRECURIT_DRY_RUN_DEFAULT                    = tostring(var.qtrecurit_dry_run_default)
    QTRECURIT_TIMEOUT_SECONDS                    = tostring(var.qtrecurit_timeout_seconds)
  }

  dynamic "oss_mount_config" {
    for_each = local.lark_cli_credentials_mount_enabled ? [1] : []
    content {
      mount_points {
        bucket_name = var.lark_cli_credentials_oss_bucket
        bucket_path = local.lark_cli_credentials_bucket_path
        endpoint    = local.lark_cli_credentials_mount_endpoint
        mount_dir   = "/home/app"
        read_only   = false
      }
    }
  }

  tags = {
    project     = var.project
    environment = var.environment
  }
}

# HTTP 触发器：直接访问（后续经 API 网关统一接入）
resource "alicloud_fcv3_trigger" "http" {
  function_name = alicloud_fcv3_function.this.function_name
  trigger_name  = "http"
  trigger_type  = "http"
  qualifier     = "LATEST"
  trigger_config = jsonencode({
    authType = "anonymous"
    methods  = ["GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS"]
  })
}
