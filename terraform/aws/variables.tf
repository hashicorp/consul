variable "platform" {
    default = "ubuntu"
    description = "The OS Platform"
}

variable "user" {
    default = {
        ubuntu = "ubuntu"
        rhel6 = "ec2-user"
        centos6 = "root"
    }
}

variable "ami" {
    description = "AWS AMI Id, if you change, make sure it is compatible with instance type, not all AMIs allow all instance types "
    default = {
        us-east-1-ubuntu = "ami-83c525e8"
        us-west-2-ubuntu = "ami-57e8d767"
        us-east-1-rhel6 = "ami-b0fed2d8"
        us-west-2-rhel6 = "ami-2faa861f"
        us-east-1-centos6 = "ami-c2a818aa"
        us-west-2-centos6 = "ami-81d092b1"
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
    default = "t2.micro"
    description = "AWS Instance type, if you change, make sure it is compatible with AMI, not all AMIs allow all instance types "
}

variable "tagName" {
    default = "consul"
    description = "Name tag for the servers"
}
