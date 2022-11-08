locals {
  client_range = var.deploy_consul_ecs ? range(1, 4) : []
  clients = tomap({ for c in local.client_range : "client${c}" => {
    index : c
    subnet_id : module.vpc.private_subnets[c - 1]
  } })
}

module "consul-clients" {
  source   = "./modules/ecs-service"
  for_each = local.clients

  name             = title(each.key)
  ecs_cluster_name = aws_ecs_cluster.ecs.name
  subnet_ids       = [each.value["subnet_id"]]
  vpc_id           = module.vpc.vpc_id

  health_check_grace_period_seconds  = 90
  deployment_maximum_percent         = 100
  deployment_minimum_healthy_percent = 0
  target_groups = {
    consul8500 : {
      protocol : "TCP"
      port : 8500
      arn : aws_lb_target_group.consul-client-tg[each.key].arn
    }
  }
  container_name = "consul"
  cpu            = 2048
  memory         = 4096

  task_definition = [
    {
      name : "consul"
      image : "${aws_ecr_repository.ecr_repository.repository_url}:agent"
      cpu : 2048
      memory : 4096
      essential : true
      command : [
        "agent", "-node=${each.key}", "-datacenter=dc1",
        "-retry-join", "provider=aws tag_key=Consul-Auto-Join tag_value=${var.cluster_name} service=ecs",
        "-encrypt=${var.consul_encryption_token}", "-https-port=8501", "-grpc-port=8502"
      ]
      environment : [
        {
          name : "CONSUL_BIND_INTERFACE"
          value : "eth1"
        },
        {
          name : "CONSUL_CLIENT_INTERFACE"
          value : "eth1"
        },
        {
          name : "CONSUL_DATA_DIR"
          value : "/consul/data"
        },
        {
          name : "CONSUL_CONFIG_DIR"
          value : "/consul/config"
        },
      ]
      linuxParameters : {
        initProcessEnabled : var.enable_container_init
      }
      portMappings : local.consul_portmap
      volumesFrom : []
      mountPoints : []
      secrets : []
      logConfiguration : {
        logDriver : "awslogs",
        options : {
          awslogs-group : aws_cloudwatch_log_group.container-logs.name,
          awslogs-region : data.aws_region.current.name,
          awslogs-stream-prefix : "agent"
        }
      }
    }
  ]
  efs_volumes = {}
  security_group_ids = [
    aws_security_group.consul.id,
  ]
  ecs_execution_role_arn = aws_iam_role.ecs_execution_role.arn
  ecs_task_role_arn      = aws_iam_role.ecs_task_role.arn
  ecs_task_role_id       = aws_iam_role.ecs_task_role.id

  depends_on = [
    aws_lb_target_group.consul-client-tg
  ]
}