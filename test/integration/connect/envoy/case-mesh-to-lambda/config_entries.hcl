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
}
