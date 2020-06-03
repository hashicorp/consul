enable_central_service_config = true

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
}
