services {
  name = "s2"
  port = 8181
  connect { 
    sidecar_service {
      proxy {
        local_service_address = "s2-sidecar-proxy"
      }
  } }
}
