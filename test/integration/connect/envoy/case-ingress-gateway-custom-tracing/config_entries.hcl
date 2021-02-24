config_entries {
  bootstrap {
    kind               = "ingress-gateway"
    name               = "ingress-gateway-random-sampling-0"
    tracing_strategy   = "random_sampling"
    tracing_percentage = 0.0
    listeners          = [{
      port     = 9990
      protocol = "http"
      services = [{
        name = "*"
      }]
    }]
  }

  bootstrap {
    kind               = "ingress-gateway"
    name               = "ingress-gateway-random-sampling-100"
    tracing_strategy   = "random_sampling"
    tracing_percentage = 100.0
    listeners          = [{
      port     = 9991
      protocol = "http"
      services = [{
        name = "*"
      }]
    }]
  }

  bootstrap {
    kind               = "ingress-gateway"
    name               = "ingress-gateway-client-sampling-0"
    tracing_strategy   = "client_sampling"
    tracing_percentage = 0.0
    listeners          = [{
      port     = 9992
      protocol = "http"
      services = [{
        name = "*"
      }]
    }]
  }

  bootstrap {
    kind               = "ingress-gateway"
    name               = "ingress-gateway-client-sampling-100"
    tracing_strategy   = "client_sampling"
    tracing_percentage = 100.0
    listeners          = [{
      port     = 9993
      protocol = "http"
      services = [{
        name = "*"
      }]
    }]
  }
}
