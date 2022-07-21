variable "name" {
  default = "efs"
}

variable "tags" {
  type = map(string)
  default = {}
}

variable "vpc_id" {
  type = string
}

variable "access_point_config" {
  default = {}
}
