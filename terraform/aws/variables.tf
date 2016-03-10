variable "platform" {
    default = "ubuntu"
    description = "The OS Platform"
}

variable "user" {
    default = {
        ubuntu  = "ubuntu"
        rhel6   = "ec2-user"
        centos6 = "centos"
        rhel7   = "ec2-user"
    }
}

variable "ami" {
    description           = "AWS AMI Id, if you change, make sure it is compatible with instance type, not all AMIs allow all instance types "
    default = {
        us-east-1-ubuntu  = "ami-fce3c696"
        us-west-2-ubuntu  = "ami-9abea4fb"
        eu-west-1-ubuntu = "ami-47a23a30"
        eu-central-1-ubuntu = "ami-accff2b1"
        ap-northeast-1-ubuntu = "ami-90815290"
        ap-southeast-1-ubuntu = "ami-0accf458"
        ap-southeast-2-ubuntu = "ami-1dc8b127"
        us-east-1-rhel6   = "ami-0d28fe66"
        us-west-2-rhel6   = "ami-3d3c0a0d"
        us-east-1-centos6 = "ami-57cd8732"
        us-west-2-centos6 = "ami-1255b321"
        us-east-1-rhel7   = "ami-2051294a"
        us-west-2-rhel7   = "ami-775e4f16"
    }
}

variable "service_conf" {
  default = {
    ubuntu  = "debian_upstart.conf"
    rhel6   = "rhel_upstart.conf"
    centos6 = "rhel_upstart.conf"
    rhel7   = "rhel_consul.service"
  }
}
variable "service_conf_dest" {
  default = {
    ubuntu  = "upstart.conf"
    rhel6   = "upstart.conf"
    centos6 = "upstart.conf"
    rhel7   = "consul.service"
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
