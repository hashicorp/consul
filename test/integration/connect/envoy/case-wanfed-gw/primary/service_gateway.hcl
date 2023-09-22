services {
  name = "mesh-gateway"
  kind = "mesh-gateway"
  port = 4431
  meta {
    consul-wan-federation = "1"
  }
}
