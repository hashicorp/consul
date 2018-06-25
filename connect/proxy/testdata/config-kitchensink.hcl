# Example proxy config with everything specified

token = "11111111-2222-3333-4444-555555555555"

proxied_service_name = "web"
proxied_service_namespace = "default"

public_listener {
  bind_address = "127.0.0.1"
  bind_port= "9999"
  local_service_address = "127.0.0.1:5000"
}

upstreams = [
  {
    local_bind_address = "127.0.0.1:6000"
    destination_name = "db"
    destination_namespace = "default"
    destination_type = "service"
  },
  {
    local_bind_address = "127.0.0.1:6001"
    destination_name = "geo-cache"
    destination_namespace = "default"
    destination_type = "prepared_query"
  }
]
