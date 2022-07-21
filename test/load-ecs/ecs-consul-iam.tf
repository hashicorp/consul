data "aws_region" "current" {}
data "aws_caller_identity" "current" {}
data "aws_kms_alias" "secretsmanager" {
  name = "alias/aws/secretsmanager"
}
#### Execution Role
data "aws_iam_policy" "ecs_execution_role_managed_policy" {
  name = "AmazonECSTaskExecutionRolePolicy"
}

data "aws_iam_policy_document" "ecs_execution_role_policy" {
  statement {
    sid     = "${title(replace(var.cluster_name, "-", ""))}TaskExecutionRolePolicy"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "ecs_execution_secrets_mounts" {
  statement {
    sid    = "${title(replace(var.cluster_name, "-", ""))}SecretsManager"
    effect = "Allow"
    actions = [
      "secretsmanager:GetSecretValue",
    ]
    resources = [
      "arn:aws:secretsmanager:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:secret:${lower(var.cluster_name)}/${lower(random_pet.test.id)}/*",
    ]
  }
  statement {
    effect = "Allow"
    actions = [
      "kms:Encrypt",
      "kms:Decrypt",
      "kms:DescribeKey",
    ]
    resources = [
      data.aws_kms_alias.secretsmanager.arn,
      data.aws_kms_alias.secretsmanager.target_key_arn,
    ]
  }
}

resource "aws_iam_role" "ecs_execution_role" {
  assume_role_policy = data.aws_iam_policy_document.ecs_execution_role_policy.json
  description        = "${title(var.cluster_name)} ECS Container Execution Role"
  name               = "${title(var.cluster_name)}TaskExecutionRole"

  inline_policy {
    name = "${title(var.cluster_name)}ExecutionSecrets"
    policy = data.aws_iam_policy_document.ecs_execution_secrets_mounts.json
  }

  managed_policy_arns = [
    data.aws_iam_policy.ecs_execution_role_managed_policy.arn
  ]
}


##### Task Role
data "aws_iam_policy_document" "ecs_task_role_policy" {
  statement {
    sid     = "${title(replace(var.cluster_name, "-", ""))}TaskRolePolicy"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
    condition {
      test     = "ArnLike"
      values   = ["arn:aws:ecs:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:*"]
      variable = "aws:SourceArn"
    }
    condition {
      test     = "StringEquals"
      values   = [data.aws_caller_identity.current.account_id]
      variable = "aws:SourceAccount"
    }
  }
}

resource "aws_iam_role" "ecs_task_role" {
  assume_role_policy = data.aws_iam_policy_document.ecs_task_role_policy.json
  description        = "${title(var.cluster_name)} ECS Container Task Role"
  name               = "${title(replace(var.cluster_name, "-", ""))}TaskRole"
}

data "aws_iam_policy_document" "ecs_efs_access" {
  count = var.deploy_efs_cluster ? 1 : 0
  statement {
    sid    = "${title(replace(var.cluster_name, "-", ""))}EfsAccessPolicy"
    effect = "Allow"
    actions = [
      "elasticfilesystem:ClientMount",
      "elasticfilesystem:ClientWrite",
      "elasticfilesystem:ClientRootAccess",
    ]
    resources = [
      module.consul-server-efs-cluster[0].efs_arn
    ]
  }
}

resource "aws_iam_role_policy" "ecs_efs_access" {
  count  = var.deploy_efs_cluster ? 1 : 0
  name   = "${title(replace(var.cluster_name, "-", ""))}EFSAccessPolicy"
  policy = data.aws_iam_policy_document.ecs_efs_access[0].json
  role   = aws_iam_role.ecs_task_role.id
}

data "aws_iam_policy_document" "ecs_auto_discover" {
  statement {
    sid    = "${title(replace(var.cluster_name, "-", ""))}EcsAutoDiscover"
    effect = "Allow"
    actions = [
      "ecs:ListClusters",
      "ecs:ListServices",
      "ecs:DescribeServices",
      "ecs:ListTasks",
      "ecs:DescribeTasks",
      "ecs:DescribeContainerInstances",
      "ec2:DescribeNetworkInterfaces",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "ecs_auto_discover" {
  name   = "${title(replace(var.cluster_name, "-", ""))}EcsAutoDiscover"
  policy = data.aws_iam_policy_document.ecs_auto_discover.json
  role   = aws_iam_role.ecs_task_role.id
}

data "aws_iam_policy_document" "ecs_datadog" {
  statement {
    sid    = "${title(replace(var.cluster_name, "-", ""))}EcsDatadog"
    effect = "Allow"
    actions = [
      "ecs:ListClusters",
      "ecs:ListContainerInstances",
      "ecs:DescribeContainerInstances",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "ecs_datadog" {
  name   = "${title(replace(var.cluster_name, "-", ""))}EcsDatadog"
  policy = data.aws_iam_policy_document.ecs_auto_discover.json
  role   = aws_iam_role.ecs_task_role.id
}

data "aws_iam_policy_document" "ecs_secretsmanager" {
  statement {
    sid    = "${title(replace(var.cluster_name, "-", ""))}SecretsManager"
    effect = "Allow"
    actions = [
      "secretsmanager:GetSecretValue",
    ]
    resources = [
      "arn:aws:secretsmanager:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:secret:${lower(var.cluster_name)}/${lower(random_pet.test.id)}/*",
    ]
  }
  statement {
    effect = "Allow"
    actions = [
      "kms:Encrypt",
      "kms:Decrypt",
      "kms:DescribeKey",
    ]
    resources = [
      data.aws_kms_alias.secretsmanager.arn,
      data.aws_kms_alias.secretsmanager.target_key_arn,
    ]
  }
}

resource "aws_iam_role_policy" "ecs_secretsmanager" {
  name   = "${title(var.cluster_name)}EcsSecretsManager"
  policy = data.aws_iam_policy_document.ecs_secretsmanager.json
  role   = aws_iam_role.ecs_task_role.id
}

### Cloudwatch
data "aws_iam_policy_document" "ecs_task_role_cloudwatch_alarm_policy" {
  statement {
    sid    = "${title(replace(var.cluster_name, "-", ""))}CloudWatchAlarmPolicy"
    effect = "Allow"
    resources = [
      "arn:aws:cloudwatch:*:*:alarm:*",
    ]
    actions = [
      "cloudwatch:DeleteAlarms",
      "cloudwatch:DescribeAlarms",
      "cloudwatch:PutMetricAlarm"
    ]
  }
}

resource "aws_iam_role_policy" "ecs_task_role_cloudwatch_alarm_policy" {
  name   = "${title(replace(var.cluster_name, "-", ""))}CloudWatchAlarmPolicy"
  role   = aws_iam_role.ecs_task_role.id
  policy = data.aws_iam_policy_document.ecs_task_role_cloudwatch_alarm_policy.json
}

## SSM Access
data "aws_iam_policy_document" "ecs_task_role_ssm_desc_policy" {
  statement {
    sid    = "${title(replace(var.cluster_name, "-", ""))}ECSTaskRoleSSMDescribePolicy"
    effect = "Allow"
    resources = [
      "*"
    ]
    actions = [
      "ssm:DescribeSessions"
    ]
  }
}

data "aws_iam_policy_document" "ecs_task_role_ssm_sess_policy" {
  statement {
    sid    = "${title(replace(var.cluster_name, "-", ""))}SSMSessionPolicy"
    effect = "Allow"
    resources = [
      "arn:aws:ecs:*:*:task/*",
      "arn:aws:ssm:*:*:document/AmazonECS-ExecuteInteractiveCommand"
    ]
    actions = [
      "ssm:StartSession"
    ]
  }

  statement {
    sid    = "${title(replace(var.cluster_name, "-", ""))}SSMChannelPolicy"
    effect = "Allow"
    resources = [
      "*",
    ]
    actions = [
      "ssm:UpdateInstanceInformation",
      "ssmmessages:CreateControlChannel",
      "ssmmessages:CreateDataChannel",
      "ssmmessages:OpenControlChannel",
      "ssmmessages:OpenDataChannel",
    ]
  }
}

data "aws_iam_policy_document" "ecs_task_role_ecr_policy" {
  statement {
    sid    = "${title(replace(var.cluster_name, "-", ""))}ECRPolicy"
    effect = "Allow"
    resources = [
      "*"
    ]
    actions = [
      "ecr:GetAuthorizationToken",
      "ecr:BatchCheckLayerAvailability",
      "ecr:GetDownloadUrlForLayer",
      "ecr:BatchGetImage",
      "logs:CreateLogStream",
      "logs:PutLogEvents"
    ]
  }
}

resource "aws_iam_role_policy" "ecs_task_role_ssm_desc_policy" {
  name_prefix = "${title(replace(var.cluster_name, "-", ""))}TaskRoleSSMDescribePolicy"
  role        = aws_iam_role.ecs_task_role.id
  policy      = data.aws_iam_policy_document.ecs_task_role_ssm_desc_policy.json
}

resource "aws_iam_role_policy" "ecs_task_role_ssm_sess_policy" {
  name_prefix = "${title(replace(var.cluster_name, "-", ""))}TaskRoleSSMSessionPolicy"
  role        = aws_iam_role.ecs_task_role.id
  policy      = data.aws_iam_policy_document.ecs_task_role_ssm_sess_policy.json
}

resource "aws_iam_role_policy" "ecs_task_role_ecr_policy" {
  name_prefix = "${title(replace(var.cluster_name, "-", ""))}TaskRoleSSMECRPolicy"
  role        = aws_iam_role.ecs_task_role.id
  policy      = data.aws_iam_policy_document.ecs_task_role_ecr_policy.json
}
