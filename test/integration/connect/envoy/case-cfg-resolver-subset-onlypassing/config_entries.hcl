config_entries {
  bootstrap {
    kind = "proxy-defaults"
    name = "global"

    config {
      protocol = "http"
    }
  }

  bootstrap {
    kind = "service-resolver"
    name = "s2"

    default_subset = "test"

    subsets = {
      "test" = {
        only_passing = true
      }
    }
  }
}
