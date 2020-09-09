# ---------------------------------------------------------------------------------------------------------------------
# Start up test servers to run tests from
# ---------------------------------------------------------------------------------------------------------------------

resource "aws_launch_configuration" "test-servers" {
  name_prefix          = "${var.cluster_name}-test-"
  image_id             = var.test_server_ami
  instance_type        = var.test_instance_type
  key_name             = module.keys.key_name
  security_groups      = [module.consul.security_group_id_clients]

  associate_public_ip_address = var.test_public_ip
  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_autoscaling_group" "test-servers" {
  name                      = aws_launch_configuration.test-servers.name
  launch_configuration      = aws_launch_configuration.test-servers.id
  min_size                  = 0
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