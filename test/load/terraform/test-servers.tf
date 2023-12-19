data "aws_ami" "test" {
  most_recent = true

  owners = var.ami_owners

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "is-public"
    values = ["false"]
  }

  filter {
    name   = "name"
    values = ["consul-test-*"]
  }
}

# ---------------------------------------------------------------------------------------------------------------------
# Start up test servers to run tests from
# ---------------------------------------------------------------------------------------------------------------------
resource "aws_security_group" "test-servers" {
  name   = "${local.random_name}-test-server-sg"
  vpc_id = module.vpc.vpc_id

  ingress {
    from_port       = 8500
    to_port         = 8500
    security_groups = [module.consul_clients.security_group_id]
    protocol        = "6"
    cidr_blocks     = [var.vpc_cidr]
  }
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "6"
    cidr_blocks = [var.vpc_allwed_ssh_cidr]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_instance" "test-server" {
  ami                         = var.test_server_ami == null ? data.aws_ami.test.id : var.test_server_ami
  instance_type               = var.test_instance_type
  key_name                    = module.keys.key_name
  vpc_security_group_ids      = toset([aws_security_group.test-servers.id])
  associate_public_ip_address = var.test_public_ip
  subnet_id                   = (module.vpc.public_subnets)[0]
  tags = {
    Name = "consul-load-generator-server-${local.random_name}"
  }
  provisioner "remote-exec" {
    inline = [
      "export LB_ENDPOINT=${module.alb.this_lb_dns_name}",
      "k6 run -q /home/ubuntu/scripts/loadtest.js"
    ]
    connection {
      type        = "ssh"
      user        = "ubuntu"
      timeout     = "1m"
      private_key = module.keys.private_key_pem
      host        = aws_instance.test-server.public_ip
    }
  }
}
