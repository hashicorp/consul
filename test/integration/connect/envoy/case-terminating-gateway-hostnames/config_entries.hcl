config_entries {
  bootstrap {
    kind = "terminating-gateway"
    name = "terminating-gateway"

    services = [
      {
        name = "s4"
      }
    ]
  }
  bootstrap {
    kind     = "service-defaults"
    name     = "s4"
    protocol = "http"
  }
}
