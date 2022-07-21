output "efs_id" {
  value = aws_efs_file_system.efs_storage.id
}

output "efs_arn" {
  value = aws_efs_file_system.efs_storage.arn
}

output "efs_name" {
  value = aws_efs_file_system.efs_storage.arn
}

output "access_point_arns" {
  value = [for p in aws_efs_access_point.efs_storage: p.arn]
}

output "access_point_ids" {
  value = [for p in aws_efs_access_point.efs_storage: p.id]
}

output "mount_target_ids" {
  value = [for t in aws_efs_mount_target.efs_storage: t.id]
}

output "efs_service_security_group_id" {
  value = aws_security_group.storage_service.id
}

output "efs_client_security_group_id" {
  value = aws_security_group.storage_client.id
}
