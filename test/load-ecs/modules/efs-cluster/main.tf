resource "aws_efs_file_system" "efs_storage" {
  encrypted = true
}

resource "aws_efs_access_point" "efs_storage" {
  for_each = var.access_point_config
  file_system_id = aws_efs_file_system.efs_storage.id
  tags = var.tags
  root_directory {
    path = "/${each.key}"
    creation_info {
      owner_gid   = each.value["owner_gid"]
      owner_uid   = each.value["owner_uid"]
      permissions = each.value["permissions"]
    }
  }
}

resource "aws_efs_mount_target" "efs_storage" {
  for_each = var.access_point_config
  file_system_id = aws_efs_file_system.efs_storage.id
  subnet_id      = each.value["subnet_id"]
  security_groups = [aws_security_group.storage_service.id]
}

resource "aws_security_group" "storage_service" {
  name = "${title(var.name)}EFSService"
  tags = var.tags
  vpc_id = var.vpc_id
}

resource "aws_security_group" "storage_client" {
  name = "${title(var.name)}EFSClient"
  tags = var.tags
  vpc_id = var.vpc_id
}

resource "aws_security_group_rule" "consul_storage_rule" {
  description = "Allow Access to EFS Access Point"
  security_group_id = aws_security_group.storage_service.id
  from_port         = 2049
  to_port           = 2049
  protocol          = "tcp"
  type              = "ingress"
  source_security_group_id = aws_security_group.storage_client.id
}
