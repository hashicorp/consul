config_entries {
  bootstrap = [
    {
      kind      = "proxy-defaults"
      name      = "global"
      partition = "default"

      config {
        protocol = "tcp"
      }
    },
    {
      kind      = "exported-services"
      name      = "default"
      partition = "default"
      services = [
        {
          name = "s2"
          consumers = [
            {
              peer_name = "alpha-to-primary"
            }
          ]
        }
      ]
    }
  ]
}
