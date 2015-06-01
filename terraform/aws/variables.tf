variable "platform" {
    default = "ubuntu"
    description = "The region of AWS, for AMI lookups."
}

variable "user" {
    default = {
        ubuntu = "ubuntu"
        rhel6 = "ec2-user"
    }
}

variable "ami" {
    default = {
        us-east-1-ubuntu = "ami-3acc7a52"
        us-west-2-ubuntu = "ami-37501207"
        us-east-1-rhel6 = "ami-b0fed2d8"
        us-west-2-rhel6 = "ami-2faa861f"
    }
}

variable "key_name" {
    description = "SSH key name in your AWS account for AWS instances."
}

variable "key_path" {
    description = "Path to the private key specified by key_name."
}

variable "region" {
    default = "us-east-1"
    description = "The region of AWS, for AMI lookups."
}

variable "servers" {
    default = "3"
    description = "The number of Consul servers to launch."
}

variable "instance_type" {
    default = {
        ubuntu = "m1.small"
        rhel6 = "m3.medium"
    }
}

variable "tagName" {
    default = "consul"
    description = "Name tag for the servers"
}

variable "platform" {
    default = "ubuntu"
    description = "The OS Platform"
}
