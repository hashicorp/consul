config_entries {
  bootstrap {
    kind = "terminating-gateway"
    name = "terminating-gateway"

    services = [
      {
        name = "l2"
      }
    ]
  }
  bootstrap {
    kind = "service-defaults",
    name = "l1",
    protocol = "http"
    meta {
      "serverless.consul.hashicorp.com/v1alpha1/lambda/enabled" = "true"
      "serverless.consul.hashicorp.com/v1alpha1/lambda/arn" = "arn:aws:lambda:us-west-2:977604411308:function:consul-envoy-integration-test"
      "serverless.consul.hashicorp.com/v1alpha1/lambda/payload-passthrough" = "true"
      "serverless.consul.hashicorp.com/v1alpha1/lambda/region" = "us-west-2"
    }
  }
  bootstrap {
    kind = "service-defaults",
    name = "l2",
    protocol = "http"
    meta {
      "serverless.consul.hashicorp.com/v1alpha1/lambda/enabled" = "true"
      "serverless.consul.hashicorp.com/v1alpha1/lambda/arn" = "arn:aws:lambda:us-west-2:977604411308:function:consul-envoy-integration-test"
      "serverless.consul.hashicorp.com/v1alpha1/lambda/payload-passthrough" = "false"
      "serverless.consul.hashicorp.com/v1alpha1/lambda/region" = "us-west-2"
    }
  }
}
