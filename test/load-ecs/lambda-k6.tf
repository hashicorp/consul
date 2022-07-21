resource "aws_secretsmanager_secret" "k6_apikey" {
  name                    = "${lower(var.cluster_name)}/${lower(random_pet.test.id)}/k6_apikey"
  recovery_window_in_days = 7
}

resource "aws_secretsmanager_secret_version" "k6_apikey" {
  secret_id     = aws_secretsmanager_secret.k6_apikey.id
  secret_string = var.k6_apikey
}

resource "aws_cloudwatch_log_group" "function-logs" {
  name              = "/aws/lambda/${var.cluster_name}K6"
  retention_in_days = 7
}

resource "aws_lambda_function" "function" {
  count         = var.deploy_consul_ecs ? 1 : 0
  function_name = "${var.cluster_name}K6"
  role          = aws_iam_role.lambda-assume.arn
  memory_size   = 256
  timeout       = 900

  source_code_hash = filebase64sha256("${path.module}/containers/k6/loadtest.js")
  image_uri = "${aws_ecr_repository.ecr_repository.repository_url}:k6"
  package_type = "Image"
  publish = true

  vpc_config {
    subnet_ids = module.vpc.private_subnets
    security_group_ids = [
      aws_security_group.k6.id
    ]
  }

  environment {
    variables = {
      LB_ENDPOINT = lower(aws_lb.admin-interface[0].dns_name)
      K6_CLOUD_TOKEN = var.k6_apikey
      K6_INSECURE_SKIP_TLS_VERIFY = true
      XDG_CONFIG_HOME = "/var/task"
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.function-logs,
    aws_iam_role.ecs_execution_role
  ]
}

resource "aws_lambda_invocation" "k6" {
  count = var.run_k6 ? 1 : 0
  function_name = "${var.cluster_name}K6"
  input = jsonencode({})
}

resource "aws_security_group" "k6" {
  name   = "${title(var.cluster_name)}K6Access"
  vpc_id = module.vpc.vpc_id
}

resource "aws_security_group_rule" "k6_egress_all" {
  security_group_id = aws_security_group.k6.id
  type              = "egress"
  protocol          = "-1"
  from_port         = 0
  to_port           = 65535
  cidr_blocks       = ["0.0.0.0/0"]
  ipv6_cidr_blocks  = ["::/0"]
}
