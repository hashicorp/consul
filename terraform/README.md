These Terraform modules were removed from GitHub in [GH-5085](https://github.com/hashicorp/consul/pull/5085).

These are not currently being maintained and tested, and were created prior to the existence of the Terraform Module Registry, which is the more appropriate way to share and distribute modules.

In an effort to limit confusion of the purpose of these modules and not encourage usage of something we aren't confident about, this removes them from this repository.

You can still access these modules if you depend on them by pinning to a specific ref in Git. It is recommended you pin against a recent major version where these modules existed:

module "consul-aws" {
  source = "git::https://github.com/hashicorp/consul.git//terraform/aws?ref=v1.4.0"
}
More detail about module sources can be found on this page:

https://www.terraform.io/docs/modules/sources.html