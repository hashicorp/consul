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
      kind = "service-resolver"
      name = "s2"
      redirect {
        service = "s3"
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
              peer_name = "alpha-to-primary"
            }
          ]
        }
      ]
    }
  ]
}
