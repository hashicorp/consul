variable "platform" {
    default = "ubuntu"
    description = "The region of AWS, for AMI lookups."
}

variable "user" {
    default = "ubuntu"
    description = "The region of AWS, for AMI lookups."
}

variable "user" {
    default = {
        ubuntu = "ubuntu"
        rhel = "ec2-user"
    }
}

variable "ami" {
    default = {
        us-east-1-ubuntu = "ami-3acc7a52"
        us-west-2-ubuntu = "ami-37501207"
        us-east-1-rhel = "ami-b0fed2d8"
        us-west-2-rhel = "ami-4dbf9e7d"
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
    default = "m3.medium"
    description = "The instance type to launch."
}

variable "tagName" {
    default = "consul"
    description = "Name tag for the servers"
}

variable "platform" {
    default = "ubuntu"
    description = "The OS Platform"
}