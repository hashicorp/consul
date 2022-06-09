config_entries {
  bootstrap = [
    {
      kind      = "proxy-defaults"
      name      = "global"
      partition = "default"

      config {
        protocol = "tcp"
      }
    }
  ]
}
