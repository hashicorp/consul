enable_central_service_config = true

acl {
  default_policy = "deny"
}

config_entries {
  bootstrap {
    kind     = "service-defaults"
    name     = "s2"
    protocol = "http"
  }

  # TODO: test header invert
  bootstrap {
    kind = "service-intentions"
    name = "s2"

    sources {
      name = "s1"
      permissions = [
        // paths
        {
          action = "allow"
          http { path_exact = "/exact" }
        },
        {
          action = "allow"
          http { path_prefix = "/prefix" }
        },
        {
          action = "allow"
          http { path_regex = "/reg[ex]{2}" }
        },
        // headers
        {
          action = "allow"
          http {
            path_exact = "/hdr-present"
            header = [{
              name    = "x-test-debug"
              present = true
            }]
          }
        },
        {
          action = "allow"
          http {
            path_exact = "/hdr-exact"
            header = [{
              name  = "x-test-debug"
              exact = "exact"
            }]
          }
        },
        {
          action = "allow"
          http {
            path_exact = "/hdr-prefix"
            header = [{
              name   = "x-test-debug"
              prefix = "prefi"
            }]
          }
        },
        {
          action = "allow"
          http {
            path_exact = "/hdr-suffix"
            header = [{
              name   = "x-test-debug"
              suffix = "uffix"
            }]
          }
        },
        {
          action = "allow"
          http {
            path_exact = "/hdr-regex"
            header = [{
              name  = "x-test-debug"
              regex = "reg[ex]{2}"
            }]
          }
        },
        // methods
        {
          action = "allow"
          http {
            path_exact = "/method-match"
            methods    = ["GET", "PUT"]
          }
        }
      ]
    }
  }
}
