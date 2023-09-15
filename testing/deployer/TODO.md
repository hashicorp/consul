Missing things that should probably be added;

- consul-dataplane support for running mesh gateways
- consul-dataplane health check updates (automatic; manual)
- ServerExternalAddresses in a peering; possibly rig up a DNS name for this.
- after creating a token, verify it exists on all servers before proceding (rather than sleep looping on not-founds)
- investigate strange gRPC bug that is currently papered over
- allow services to override their mesh gateway modes
- remove some of the debug prints of various things
