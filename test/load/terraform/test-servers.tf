# ---------------------------------------------------------------------------------------------------------------------
# Start up test servers to run tests from
# ---------------------------------------------------------------------------------------------------------------------
resource "aws_security_group" "test-servers" {
  name   = "${local.random_name}-test-server-sg"
  vpc_id = module.vpc.vpc_id

  ingress {
    from_port       = 8500
    to_port         = 8500
    security_groups = [module.consul.security_group_id_clients]
    protocol        = "6"
    cidr_blocks     = ["0.0.0.0/0"]
  }
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "6"
    cidr_blocks = ["0.0.0.0/0"]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_launch_configuration" "test-servers" {
  name_prefix     = "${var.cluster_name}-${local.random_name}-test-"
  image_id        = var.test_server_ami
  instance_type   = var.test_instance_type
  key_name        = module.keys.key_name
  security_groups = [aws_security_group.test-servers.id]

  associate_public_ip_address = var.test_public_ip
  lifecycle {
    create_before_destroy = true
  }
  user_data = templatefile(
    "./start-k6.sh",
    {
      lb_endpoint = module.alb.this_lb_dns_name
    }
  )
}

resource "aws_autoscaling_group" "test-servers" {
  name                      = aws_launch_configuration.test-servers.name
  launch_configuration      = aws_launch_configuration.test-servers.id
  min_size                  = 2
  max_size                  = 5
  desired_capacity          = 2
  wait_for_capacity_timeout = "480s"
  health_check_grace_period = 15
  health_check_type         = "EC2"
  vpc_zone_identifier       = module.vpc.public_subnets

  lifecycle {
    create_before_destroy = true
  }
}
