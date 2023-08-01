# Cluster membership

This section is a work in progress. It will contain topics like the following:

   - hashicorp/serf
   - hashicorp/memberlist
   - network coordinates
   - consul events
   - consul exec


Both client and server mode agents participate in a [Gossip Protocol](https://developer.hashicorp.com/consul/docs/architecture/gossip) which provides two important mechanisms. First, it allows for agents to learn about all the other agents in the cluster, just by joining initially with a single existing member of the cluster. This allows clients to discover new Consul servers. Second, the gossip protocol provides a distributed failure detector, whereby the agents in the cluster randomly probe each other at regular intervals. Because of this failure detector, Consul can run health checks locally on each agent and just sent edge-triggered updates when the state of a health check changes, confident that if the agent dies altogether then the cluster will detect that. This makes Consul's health checking design very scaleable compared to centralized systems with a central polling type of design.
