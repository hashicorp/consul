config_entries {
  bootstrap = [
    {
      kind = "proxy-defaults"
      name = "global"

      config {
        protocol = "tcp"
      }
    },
    {
      kind = "exported-services"
      name = "default"
      services = [
        {
          name = "s2"
          consumers = [
            {
              peer = "alpha-to-primary"
            }
          ]
        }
      ]
    }
  ]
}
