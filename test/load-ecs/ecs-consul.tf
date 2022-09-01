locals {
  certs = {
    CONSUL_CA : "tls/consul-agent-ca.pem"
    CONSUL_SERVER_KEY : "tls/dc1-server-consul-0-key.pem"
    CONSUL_SERVER_CERT : "tls/dc1-server-consul-0.pem"
    CONSUL_CLIENT_KEY : "tls/dc1-server-consul-0-key.pem"
    CONSUL_CLIENT_CERT : "tls/dc1-server-consul-0.pem"
  }
  consul_range = var.deploy_consul_ecs ? range(1, 4) : []
  consul = tomap({ for c in local.consul_range : "consul${c}" => {
    index : c
    subnet_id : module.vpc.private_subnets[c - 1]
    owner_gid : 1000
    owner_uid : 100
    permissions : "0700"
  } })
  consul_portmap = [{
    containerPort : 8300,
    hostPort : 8300,
    protocol : "tcp"
    }, {
    containerPort : 8301,
    hostPort : 8301,
    protocol : "tcp"
    }, {
    containerPort : 8302,
    hostPort : 8302,
    protocol : "tcp"
    }, {
    containerPort : 8500,
    hostPort : 8500,
    protocol : "tcp"
    }, {
    containerPort : 8501,
    hostPort : 8501,
    protocol : "tcp"
    }, {
    containerPort : 8502,
    hostPort : 8502,
    protocol : "tcp"
    }, {
    containerPort : 8600,
    hostPort : 8600,
    protocol : "udp"
  }]
  datadog_portmap = [{
    containerPort : 8125,
    hostPort : 8125,
    protocol : "udp"
  }]
}

resource "aws_ecr_repository" "ecr_repository" {
  name                 = "${lower(var.cluster_name)}/consul"
  image_tag_mutability = "MUTABLE"
}

resource "aws_secretsmanager_secret" "gossip_token" {
  name                    = "${lower(var.cluster_name)}/${lower(random_pet.test.id)}/gossip_token"
  recovery_window_in_days = 7
}

resource "aws_secretsmanager_secret_version" "gossip_token" {
  secret_id     = aws_secretsmanager_secret.gossip_token.id
  secret_string = var.consul_encryption_token
}

resource "aws_secretsmanager_secret" "datadog_apikey" {
  name                    = "${lower(var.cluster_name)}/${lower(random_pet.test.id)}/datadog_apikey"
  recovery_window_in_days = 7
}

resource "aws_secretsmanager_secret_version" "datadog_apikey" {
  secret_id     = aws_secretsmanager_secret.datadog_apikey.id
  secret_string = var.datadog_apikey == "" ? " " : var.datadog_apikey # secretsmanager wont allow and empty string
}

resource "aws_secretsmanager_secret" "certs" {
  for_each                = local.certs
  name                    = "${lower(var.cluster_name)}/${lower(random_pet.test.id)}/${each.key}"
  recovery_window_in_days = 7
}

resource "aws_secretsmanager_secret_version" "certs" {
  for_each      = local.certs
  secret_id     = aws_secretsmanager_secret.certs[each.key].id
  secret_string = templatefile("${path.root}/${each.value}", {})
}

resource "aws_cloudwatch_log_group" "container-logs" {
  name              = "/ecs/${var.cluster_name}"
  retention_in_days = 7
}

resource "aws_ecs_cluster" "ecs" {
  name = var.cluster_name

  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

module "consul-server" {
  source   = "./modules/ecs-service"
  for_each = local.consul

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
      arn : aws_lb_target_group.consul-server-tg[each.key].arn
    }
  }
  container_name = "consul"
  sidecar_name   = "datadog"
  cpu            = 2048
  memory         = 4096

  tags = {
    "Consul-Auto-Join" : var.cluster_name
  }

  task_definition = [
    {
      name : "consul"
      image : "${aws_ecr_repository.ecr_repository.repository_url}:server"
      cpu : 1792
      memory : 3584
      essential : true
      command : [
        "agent", "-server", "-ui", "-node=${each.key}", "-datacenter=dc1", "-bootstrap-expect=3",
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
          name : "CONSUL_LOCAL_CONFIG"
          value : "{\"skip_leave_on_interrupt\": true}"
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
      mountPoints : var.deploy_efs_cluster ? [{
        containerPath : "/consul/data"
        sourceVolume : each.key
        readOnly : false
      }] : []
      secrets : [
        {
          name : "CONSUL_CA",
          valueFrom : aws_secretsmanager_secret.certs["CONSUL_CA"].arn,
        },
        {
          name : "CONSUL_CERT",
          valueFrom : aws_secretsmanager_secret.certs["CONSUL_SERVER_CERT"].arn,
        },
        {
          name : "CONSUL_KEY",
          valueFrom : aws_secretsmanager_secret.certs["CONSUL_SERVER_KEY"].arn,
        },
      ]
      logConfiguration : {
        logDriver : "awslogs",
        options : {
          awslogs-group : aws_cloudwatch_log_group.container-logs.name,
          awslogs-region : data.aws_region.current.name,
          awslogs-stream-prefix : "consul"
        }
      }
      }, {
      name : "datadog"
      image : "${aws_ecr_repository.ecr_repository.repository_url}:datadog"
      cpu : 256
      memory : 512
      essential : false
      linuxParameters : {
        initProcessEnabled : var.enable_container_init
      }
      portMappings : local.datadog_portmap
      environment : [
        {
          name : "ECS_FARGATE"
          value : "true"
        },
        {
          name : "DD_DOGSTATSD_NON_LOCAL_TRAFFIC"
          value : "true"
        },
      ]
      volumesFrom : []
      mountPoints : []
      secrets : [
        {
          name : "DD_API_KEY",
          valueFrom : aws_secretsmanager_secret.datadog_apikey.arn
        }
      ]
      logConfiguration : {
        logDriver : "awslogs",
        options : {
          awslogs-group : aws_cloudwatch_log_group.container-logs.name,
          awslogs-region : data.aws_region.current.name,
          awslogs-stream-prefix : "datadog"
        }
      }
    }
  ]
  efs_volumes = var.deploy_efs_cluster ? {
    (each.key) : {
      file_system_id : module.consul-server-efs-cluster[0].efs_id
      access_point_id : module.consul-server-efs-cluster[0].access_point_ids[each.value["index"] - 1]
      root_directory : "/"
      transit_encryption : "ENABLED"
      encryption_port : 2049
      iam : "DISABLED"
    }
  } : {}
  security_group_ids = compact([
    aws_security_group.consul.id,
    var.deploy_efs_cluster ? module.consul-server-efs-cluster[0].efs_client_security_group_id : null,
  ])
  ecs_execution_role_arn = aws_iam_role.ecs_execution_role.arn
  ecs_task_role_arn      = aws_iam_role.ecs_task_role.arn
  ecs_task_role_id       = aws_iam_role.ecs_task_role.id

  depends_on = [
    aws_lb_target_group.consul-server-tg
  ]
}

resource "aws_security_group" "consul" {
  name   = "${title(var.cluster_name)}ConsulAccess"
  vpc_id = module.vpc.vpc_id
}

resource "aws_security_group_rule" "consul_client_ingress_8300" {
  description       = "${title(var.cluster_name)} Consul Client RPC Communication"
  security_group_id = aws_security_group.consul.id
  type              = "ingress"
  protocol          = "tcp"
  from_port         = 8300
  to_port           = 8302
  self              = true
}

resource "aws_security_group_rule" "consul_client_ingress_8300_udp" {
  description       = "${title(var.cluster_name)} Consul Client RPC Communication"
  security_group_id = aws_security_group.consul.id
  type              = "ingress"
  protocol          = "udp"
  from_port         = 8300
  to_port           = 8302
  self              = true
}

resource "aws_security_group_rule" "consul_client_ingress_8600" {
  description       = "${title(var.cluster_name)} Consul DNS TCP Communication"
  security_group_id = aws_security_group.consul.id
  type              = "ingress"
  protocol          = "-1"
  from_port         = 8600
  to_port           = 8600
  self              = true
}

resource "aws_security_group_rule" "consul_alb_ingress_8500" {
  description              = "${title(var.cluster_name)} Consul Admin Http[s] Communication"
  security_group_id        = aws_security_group.consul.id
  type                     = "ingress"
  from_port                = 8500
  to_port                  = 8501
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.lb-access.id
}

resource "aws_security_group_rule" "consul_ingress_sidecar" {
  description       = "${title(var.cluster_name)} Consul Sidecar Communication"
  security_group_id = aws_security_group.consul.id
  type              = "ingress"
  protocol          = "tcp"
  from_port         = 21000
  to_port           = 21255
  self              = true
}

resource "aws_security_group_rule" "consul_egress_all" {
  security_group_id = aws_security_group.consul.id
  type              = "egress"
  protocol          = "-1"
  from_port         = 0
  to_port           = 65535
  cidr_blocks       = ["0.0.0.0/0"]
  ipv6_cidr_blocks  = ["::/0"]
}
