config_entries {
  bootstrap = [
    {
      kind = "proxy-defaults"
      name = "global"

      config {
        # This shouldn't affect the imported listener's protocol, which should be http.
        protocol = "tcp"
      }
    }
  ]
}
