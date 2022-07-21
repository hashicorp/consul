module "consul-server-efs-cluster" {
  source = "./modules/efs-cluster"

  name   = "${title(var.cluster_name)}FS"
  count  = var.deploy_efs_cluster ? 1 : 0
  vpc_id = module.vpc.vpc_id

  access_point_config = { for k, v in local.consul : k => {
    owner_gid : v["owner_gid"]
    owner_uid : v["owner_uid"]
    permissions : v["permissions"]
    subnet_id : module.vpc.private_subnets[v["index"] - 1]
  } }
}
