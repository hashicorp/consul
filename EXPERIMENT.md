# Skaffold Experiment with Consul

## Purpose
To shorten the feedback loop of the Consul development cycle, starting with a focus/integration on K8S.

## Non-purpose
1. Simulate all deployments of Consul
1. Create apps to deploy along with Consul in this mode

## How to use the thing

1. Make sure you have a kubernetes cluster and cli accoutrements installed and configured properly.
Tools I recommend when in doubt:
   1. [Rancher Desktop](https://rancherdesktop.io/) with the Moby container runtime selected. This will also install 
   1. [k3d](https://k3d.io/) for creating a cluster with multiple nodes, which is useful for deployments in Consul that have anti-affinity. 
   I use this one for testing. I use this for testing: `k3d cluster create testing --agents=3 --no-lb`
1. Install Skaffold[https://skaffold.dev/docs/install/]
    1. Easy mode: `brew install skaffold`
1. Clone this branch `dans/skaffold-experiment`
1. Initialize the submodule used for `consul-k8s`:
   1. `git submodule init`
   1. `git submodule update`


## Open Questions
1. If you redeploy Consul, when/how/can the sidecar proxies for services in the cluster also be redeployed.

## What's Left
1. Build the UI dynamically
1. Port Forward the Consul UI
1. There are definitely paths missing from the dependency trees