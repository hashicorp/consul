resource "aws_lb" "admin-interface" {
  count    = var.deploy_consul_ecs ? 1 : 0
  name     = "${title(var.cluster_name)}LB"
  internal = var.internal_alb_listener
  subnets  = var.internal_alb_listener ? module.vpc.private_subnets : module.vpc.public_subnets

  load_balancer_type = "application"
  security_groups    = [aws_security_group.lb-access.id]
}

resource "aws_iam_server_certificate" "alb-cert" {
  certificate_body = aws_secretsmanager_secret_version.certs["CONSUL_SERVER_CERT"].secret_string
  private_key      = aws_secretsmanager_secret_version.certs["CONSUL_SERVER_KEY"].secret_string
}

resource "aws_lb_listener" "ConsulAdmin18500" {
  count             = var.deploy_consul_ecs ? 1 : 0
  load_balancer_arn = aws_lb.admin-interface[0].arn
  port              = 18500
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-FS-1-2-2019-08"
  certificate_arn   = aws_iam_server_certificate.alb-cert.arn

  default_action {
    type = "forward"
    forward {
      stickiness {
        duration = 1
      }
      dynamic "target_group" {
        for_each = aws_lb_target_group.consul-server-tg
        content {
          arn = aws_lb_target_group.consul-server-tg[target_group.key].arn
        }
      }
    }
  }
  depends_on = [
    aws_iam_server_certificate.alb-cert
  ]
}

resource "aws_lb_target_group" "consul-server-tg" {
  for_each    = local.consul
  name        = title(each.key)
  vpc_id      = module.vpc.vpc_id
  protocol    = "HTTP"
  port        = "8500"
  target_type = "ip"
  slow_start  = 90

  health_check {
    enabled           = true
    path              = "/v1/status/leader"
    timeout           = 2
    interval          = 5
    protocol          = "HTTP"
    healthy_threshold = 2
  }
}

resource "aws_lb_listener" "ConsulAdmin8500" {
  count             = var.deploy_consul_ecs ? 1 : 0
  load_balancer_arn = aws_lb.admin-interface[0].arn
  port              = 8500
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-FS-1-2-2019-08"
  certificate_arn   = aws_iam_server_certificate.alb-cert.arn

  default_action {
    type = "forward"
    forward {
      stickiness {
        duration = 1
      }
      dynamic "target_group" {
        for_each = aws_lb_target_group.consul-client-tg
        content {
          arn = aws_lb_target_group.consul-client-tg[target_group.key].arn
        }
      }
    }
  }
  depends_on = [
    aws_iam_server_certificate.alb-cert
  ]
}

resource "aws_lb_target_group" "consul-client-tg" {
  for_each    = local.clients
  name        = title(each.key)
  vpc_id      = module.vpc.vpc_id
  protocol    = "HTTP"
  port        = "8500"
  target_type = "ip"
  slow_start  = 90

  health_check {
    enabled           = true
    path              = "/v1/status/leader"
    timeout           = 2
    interval          = 5
    protocol          = "HTTP"
    healthy_threshold = 2
  }
}

resource "aws_security_group" "lb-access" {
  name        = "${var.cluster_name}AdminLBAccess"
  description = "Allow access to ${title(var.cluster_name)} Admin LB"
  vpc_id      = module.vpc.vpc_id
}

resource "aws_security_group_rule" "alb-consul-admin-outbound-access" {
  count                    = length(var.admin_cidrs) > 0 ? 1 : 0
  from_port                = 0
  to_port                  = 65536
  protocol                 = "-1"
  type                     = "egress"
  security_group_id        = aws_security_group.lb-access.id
  cidr_blocks              = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "consul-admin-client-ingress" {
  count             = length(var.admin_cidrs) > 0 ? 1 : 0
  description       = "Allow access to ${title(var.cluster_name)} Admin LB Consul Client Ingress"
  from_port         = 8500
  to_port           = 8501
  protocol          = "tcp"
  type              = "ingress"
  security_group_id = aws_security_group.lb-access.id
  cidr_blocks       = var.admin_cidrs
}

#resource "aws_security_group_rule" "consul-lan-client-ingress" {
#  description              = "Allow access to ${title(var.cluster_name)} Admin LB Consul Server Ingress"
#  from_port                = 8500
#  to_port                  = 8501
#  protocol                 = "tcp"
#  type                     = "ingress"
#  security_group_id        = aws_security_group.lb-access.id
#  source_security_group_id = aws_security_group.consul.id
#}

resource "aws_security_group_rule" "consul-lan-k6-ingress" {
  description              = "Allow access to ${title(var.cluster_name)} Admin LB K6 Ingress"
  from_port                = 8500
  to_port                  = 8501
  protocol                 = "tcp"
  type                     = "ingress"
  security_group_id        = aws_security_group.lb-access.id
  source_security_group_id = aws_security_group.k6.id
}

resource "aws_security_group_rule" "consul-admin-server-ingress" {
  count             = length(var.admin_cidrs) > 0 ? 1 : 0
  description       = "Allow access to ${title(var.cluster_name)} Admin LB Consul Server Ingress"
  from_port         = 18500
  to_port           = 18501
  protocol          = "tcp"
  type              = "ingress"
  security_group_id = aws_security_group.lb-access.id
  cidr_blocks       = var.admin_cidrs
}
