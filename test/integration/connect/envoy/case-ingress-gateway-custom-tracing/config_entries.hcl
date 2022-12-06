config_entries {
  bootstrap {
    kind     = "service-defaults"
    name     = "s1"
    protocol = "http"
  }

  // all at 100%
  // curl with and without trace header should be traced
  bootstrap {
    kind               = "ingress-gateway"
    name               = "ingress-gateway-all-0"
    tracing {
      client_sampling = 100.0
      random_sampling = 100.0
      overall_sampling = 100.0
    }
    listeners          = [{
      port     = 9990
      protocol = "http"
      services = [{
        name = "s1"
        hosts = ["localhost:9990"]
      }]
    }]
  }

  // random @ 0 and client @ 100, overall @ 100, should be traced when trace header provided
  // curl with and without trace header should not traced
  bootstrap {
    kind               = "ingress-gateway"
    name               = "ingress-gateway-client-100"
    tracing {
      client_sampling = 100.0
      random_sampling = 0.0
      overall_sampling = 100.0
    }
    listeners          = [{
      port     = 9991
      protocol = "http"
      services = [{
        name = "s1"
        hosts = ["localhost:9991"]
      }]
    }]
  }

  // random and client @ 100, overall @ 0, should not be traced (overall acts as upper bound)
  // curl with and without trace header should not traced
  bootstrap {
    kind               = "ingress-gateway"
    name               = "ingress-gateway-overall-0"
    tracing {
      client_sampling = 100.0
      random_sampling = 100.0
      overall_sampling = 0.0
    }
    listeners          = [{
      port     = 9992
      protocol = "http"
      services = [{
        name = "s1"
        hosts = ["localhost:9992"]
      }]
    }]
  }

  // random and client @ 0, overall @ 100, should not be traced
  // curl with and without trace header should not traced
  bootstrap {
    kind               = "ingress-gateway"
    name               = "ingress-gateway-overall-100"
    tracing {
      client_sampling = 0.0
      random_sampling = 0.0
      overall_sampling = 100.0
    }
    listeners          = [{
      port     = 9993
      protocol = "http"
      services = [{
        name = "s1"
        hosts = ["localhost:9993"]
      }]
    }]
  }
}