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
          name = "s1"
          consumers = [
            {
              peer_name = "alpha-to-primary"
            }
          ]
        },
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
