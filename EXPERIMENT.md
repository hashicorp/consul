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
1. Install [Skaffold](https://skaffold.dev/docs/install/) >=v2.0
    1. Easy mode: `brew install skaffold`
1. Clone this branch `dans/skaffold-experiment`
1. Initialize the submodule used for `consul-k8s`:
   1. `git submodule init`
   1. `git submodule update`
1. Make sure you have a registry available with push/pull privledges, like a personal account on dockerhub. 
    1. Make sure you have `docker login` 
    1. Replace the registry name on Line 9 of `skaffold.yaml` with your name
    1. *NOTE: this is a temporary requirement because the [local image copy-to-cluster functionality is broken](https://github.com/GoogleContainerTools/skaffold/issues/7992).
1. Run the following command.
    1. `skaffold dev -default-repo <your repo>`
    1. *NOTE: drop the `default-repo` when the above bug is fixed
1. ☕️
1. Change a file affecting either Consul core or the K8S control plane and watch the helm chart redeploy
1. Visit the Consul UI at [http://localhost:8080](http://localhost:8080)
1. When you're done, CTRL+C to exit skaffold and `skaffold delete` to remove the helm release from your cluster.


## Open Questions
1. If you redeploy Consul, when/how/can the sidecar proxies for services in the cluster also be redeployed.

## What's Left for MVP
1. Need to repeat the process for the controller
1. Find a compelling demo

## What would be nice to have
1. See how the skaffold community manages sidecar re-deployment
1. Build the UI dynamically
1. There are definitely paths missing from the dependency trees
1. Optimize build by copying consul binary directly into container?

## Future
1. Re-enable local development without a registry when the [bug is fixed in skaffold](https://github.com/GoogleContainerTools/skaffold/issues/7992)
1. Make consul build faster
1. Make applications re-deploy sidecars

