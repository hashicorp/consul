resource "aws_instance" "server" {
    ami = "${lookup(var.ami, "${var.region}-${var.platform}")}"
    instance_type = "${var.instance_type}"
    key_name = "${var.key_name}"
    count = "${var.servers}"
    vpc_security_group_ids = ["${aws_security_group.consul.id}"]
    subnet_id = "${lookup(var.subnets, count.index % var.servers)}"
    iam_instance_profile   = "${aws_iam_instance_profile.consul-join.name}"


    connection {
        user = "${lookup(var.user, var.platform)}"
        private_key = "${file("${var.key_path}")}"
    }

    tags = "${map(
       "Name", "consul-server-${count.index}",
       var.consul_join_tag_key, var.consul_join_tag_value
    )}"

    provisioner "file" {
        source = "${path.module}/../shared/scripts/${lookup(var.service_conf, var.platform)}"
        destination = "/tmp/${lookup(var.service_conf_dest, var.platform)}"
    }

    provisioner "file" {
        source      = "${path.module}/../shared/scripts/install.sh",
        destination = "/tmp/install.sh"
    }

    provisioner "remote-exec" {
      inline = [
        "chmod +x /tmp/install.sh",
        "/tmp/install.sh ${var.servers} ${var.consul_version} ${var.consul_bind} ${var.consul_client_bind} ${var.consul_join_tag_key} ${var.consul_join_tag_value} ${var.consul_datacenter}",

      ]
    }

    provisioner "remote-exec" {
        scripts = [
            "${path.module}/../shared/scripts/service.sh",
            "${path.module}/../shared/scripts/ip_tables.sh",
        ]
    }
}

resource "aws_security_group" "consul" {
    name = "consul_${var.platform}"
    description = "Consul internal traffic + maintenance."
    vpc_id = "${var.vpc_id}"

    // These are for internal traffic
    ingress {
        from_port = 0
        to_port = 65535
        protocol = "tcp"
        self = true
    }

    ingress {
        from_port = 0
        to_port = 65535
        protocol = "udp"
        self = true
    }

    // These are for maintenance
    ingress {
        from_port = 22
        to_port = 22
        protocol = "tcp"
        cidr_blocks = ["${var.client_access_subnet}"]
    }

    ingress {
        from_port = 8500
        to_port = 8500
        protocol = "tcp"
        cidr_blocks = ["${var.client_access_subnet}"]
    }

    ingress {
        protocol = "icmp"
	cidr_blocks = ["0.0.0.0/0"]
	from_port = 8
	to_port = 0
    }


    // This is for outbound internet access
    egress {
        from_port = 0
        to_port = 0
        protocol = "-1"
        cidr_blocks = ["0.0.0.0/0"]
    }
}

# Create an IAM role for the auto-join
resource "aws_iam_role" "consul-join" {
  name               = "${var.platform}-consul-join"
  assume_role_policy = "${file("${path.module}/policies/assume-role.json")}"
}

# Create the policy
resource "aws_iam_policy" "consul-join" {
  name        = "${var.platform}-consul-join"
  description = "Allows Consul nodes to describe instances for joining."
  policy      = "${file("${path.module}/policies/describe-instances.json")}"
}

# Attach the policy
resource "aws_iam_policy_attachment" "consul-join" {
  name       = "${var.platform}-consul-join"
  roles      = ["${aws_iam_role.consul-join.name}"]
  policy_arn = "${aws_iam_policy.consul-join.arn}"
}

# Create the instance profile
resource "aws_iam_instance_profile" "consul-join" {
  name  = "${var.platform}-consul-join"
  role = "${aws_iam_role.consul-join.name}"
}


