variable "platform" {
  default     = "ubuntu"
  description = "The OS Platform"
}

variable "user" {
  default = {
    ubuntu  = "ubuntu"
    rhel6   = "ec2-user"
    centos6 = "centos"
    centos7 = "centos"
    rhel7   = "ec2-user"
  }
}

variable "ami" {
  description = "AWS AMI Id, if you change, make sure it is compatible with instance type, not all AMIs allow all instance types "

  default = {
    ap-south-1-ubuntu	  = "ami-08a5e367"
    us-east-1-ubuntu	  = "ami-d651b8ac"
    ap-northeast-1-ubuntu = "ami-8422ebe2"
    eu-west-1-ubuntu	  = "ami-17d11e6e"
    ap-southeast-1-ubuntu = "ami-e6d3a585"
    ca-central-1-ubuntu	  = "ami-e59c2581"
    us-west-1-ubuntu	  = "ami-2d5c6d4d"
    eu-central-1-ubuntu	  = "ami-5a922335"
    sa-east-1-ubuntu	  = "ami-a3e39ecf"
    ap-southeast-2-ubuntu = "ami-391ff95b"
    eu-west-2-ubuntu	  = "ami-e1f2e185"
    ap-northeast-2-ubuntu = "ami-0f6fb461"
    us-west-2-ubuntu	  = "ami-ecc63a94"
    us-east-2-ubuntu  	  = "ami-9686a4f3"
    us-east-1-rhel6       = "ami-0d28fe66"
    us-east-2-rhel6       = "ami-aff2a9ca"
    us-west-2-rhel6       = "ami-3d3c0a0d"
    us-east-1-centos6     = "ami-57cd8732"
    us-east-2-centos6     = "ami-c299c2a7"
    us-west-2-centos6     = "ami-1255b321"
    us-east-1-rhel7       = "ami-2051294a"
    us-east-2-rhel7       = "ami-0a33696f"
    us-west-2-rhel7       = "ami-775e4f16"
    us-east-1-centos7     = "ami-6d1c2007"
    us-east-2-centos7     = "ami-6a2d760f"
    us-west-1-centos7     = "ami-af4333cf"
  }
}

variable "service_conf" {
  default = {
    ubuntu  = "debian_consul.service"
    rhel6   = "rhel_upstart.conf"
    centos6 = "rhel_upstart.conf"
    centos7 = "rhel_consul.service"
    rhel7   = "rhel_consul.service"
  }
}

variable "service_conf_dest" {
  default = {
    ubuntu  = "consul.service"
    rhel6   = "upstart.conf"
    centos6 = "upstart.conf"
    centos7 = "consul.service"
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
  default     = "us-east-1"
  description = "The region of AWS, for AMI lookups."
}

variable "servers" {
  default     = "3"
  description = "The number of Consul servers to launch."
}

variable "instance_type" {
  default     = "t2.micro"
  description = "AWS Instance type, if you change, make sure it is compatible with AMI, not all AMIs allow all instance types "
}

variable "tagName" {
  default     = "consul"
  description = "Name tag for the servers"
}

variable "subnets" {
  type = "map"
  description = "map of subnets to deploy your infrastructure in, must have as many keys as your server count (default 3), -var 'subnets={\"0\"=\"subnet-12345\",\"1\"=\"subnets-23456\"}' "
}

variable "vpc_id" {
  type = "string"
  description = "ID of the VPC to use - in case your account doesn't have default VPC"
}