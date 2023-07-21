# Grafana Publishing

The page for publishing this dashboard is https://grafana.com/grafana/dashboards/13396

## How to edit and Publish
    - Start grafana locally (consul-docker-test/mesh-gateways-l7 will provide you two Consul DCs, grafana, and prometheus)_
    - Import the dashboard json into a new dashboard
    - Make changes
    - Click "share" and export for external publishing
    - Login to Grafana via the team account (message a manager)
    - Publish as a new version, including change notes.

### Grafana dashboard for consul-k8s (control plane)

A grafana dashboard for monitoring consul-k8s (control plane) can also be found in this directory: `consul-k8s-control-plane-monitoring.json`. This dashboard has not been published to https://grafana.com.
