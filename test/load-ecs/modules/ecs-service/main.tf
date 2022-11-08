resource "aws_ecs_task_definition" "task_def" {
  family = var.name

  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  execution_role_arn       = var.ecs_execution_role_arn
  task_role_arn            = var.ecs_task_role_arn
  tags                     = var.tags
  cpu                      = var.cpu
  memory                   = var.memory

  container_definitions = jsonencode(var.task_definition)

  dynamic "volume" {
    for_each = var.efs_volumes
    content {
      name = volume.key
      efs_volume_configuration {
        file_system_id = volume.value["file_system_id"]
        root_directory = volume.value["root_directory"]
        transit_encryption = volume.value["transit_encryption"]
        transit_encryption_port = volume.value["encryption_port"]
        authorization_config {
          access_point_id = volume.value["access_point_id"]
          iam             = volume.value["iam"]
        }
      }
    }
  }
}

resource "aws_ecs_service" "task_service" {
  name            = var.name
  cluster         = var.ecs_cluster_name
  task_definition = aws_ecs_task_definition.task_def.arn
  tags            = var.tags
  propagate_tags  = "SERVICE"
  desired_count   = var.desired_count
  launch_type     = "FARGATE"

  deployment_maximum_percent         = var.deployment_maximum_percent
  deployment_minimum_healthy_percent = var.deployment_minimum_healthy_percent
  health_check_grace_period_seconds  = var.health_check_grace_period_seconds

  enable_execute_command = true

  deployment_controller {
    type = "ECS"
  }

  deployment_circuit_breaker {
    enable   = var.deployment_circuit_breaker_enable
    rollback = false
  }

  network_configuration {
    assign_public_ip = var.assign_public_ip
    subnets          = var.subnet_ids
    security_groups  = var.security_group_ids
  }

  dynamic "load_balancer" {
    for_each = var.target_groups
    content {
      target_group_arn = load_balancer.value["arn"]
      container_name   = var.container_name
      container_port   = load_balancer.value["port"]
    }
  }
}
