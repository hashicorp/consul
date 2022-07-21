resource "aws_cloudwatch_dashboard" "consul-ecs-efs" {
  dashboard_name = "${var.cluster_name}-dashboard"
  dashboard_body = templatefile("${path.module}/templates/consul-ecs-efs-dashboard.json.j2", {
    aws_region : var.aws_region
    cluster_name : var.cluster_name
    efs_filesystem_id : var.deploy_efs_cluster ? module.consul-server-efs-cluster[0].efs_id : "does-not-exist"
  })
}
