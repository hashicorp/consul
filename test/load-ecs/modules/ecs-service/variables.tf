variable "name" { default = "container" }
variable "tags" {
  description = "Tags for the resources"
  type = map(string)
  default = {
    env: "dev"
  }
}

variable "ecs_cluster_name" {
  description = "ECS Cluster Name to Deploy this container and launch template"
  type = string
}

variable "ecs_execution_role_arn" {
  type = string
}

variable "ecs_task_role_arn" {
  type = string
}

variable "ecs_task_role_id" {
  type = string
}

variable "vpc_id" {
  description = "VPC ID the subnets are attached to"
  type = string
}

variable "subnet_ids" {
  description = "List of subnet ids to deploy the containers in"
  type = list(string)
}

variable "desired_count" {
  description = "ECS Service Fixed Count"
  type = number
  default = 1
}

variable "cpu" {
  default = 256
}

variable "memory" {
  default = 512
}

variable "deployment_maximum_percent" {
  default = 200
}
variable "deployment_minimum_healthy_percent" {
  default = 100
}
variable "deployment_circuit_breaker_enable" {
  default = false
}

variable "security_group_ids" {
  description = "Additional Security Group IDs to add to the Service"
  type = list(string)
  default = []
}

variable "secrets" {
  type = list(object({name: string, valueFrom: string}))
  default = []
}

variable "task_definition" {
  default = {}
}

variable "efs_volumes" {
  type = map(object({
    file_system_id: string
    access_point_id: string
    root_directory: string
    transit_encryption: string
    encryption_port: number
    iam: string
  }))
  default = {}
}

variable "container_name" {
  type = string
  default = ""
}

variable "sidecar_name" {
  type = string
  default = ""
}

variable "target_groups" {
  default = {}
}

variable "health_check_grace_period_seconds" {
  default = 0
}

variable "assign_public_ip" {
  default = false
}

variable "ignore_changes" {
  default = []
}
