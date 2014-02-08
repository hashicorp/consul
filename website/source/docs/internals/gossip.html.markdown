---
layout: "docs"
page_title: "Gossip Protocol"
sidebar_current: "docs-internals-gossip"
---

# Gossip Protocol

Serf uses a [gossip protocol](http://en.wikipedia.org/wiki/Gossip_protocol)
to broadcast messages to the cluster. This page documents the details of
this internal protocol. The gossip protocol is based on
["SWIM: Scalable Weakly-consistent Infection-style Process Group Membership Protocol"](http://www.cs.cornell.edu/~asdas/research/dsn02-swim.pdf),
with a few minor adaptations, mostly to increase propagation speed
and convergence rate.

<div class="alert alert-block alert-warning">
<strong>Advanced Topic!</strong> This page covers the technical details of
the internals of Serf. You don't need to know these details to effectively
operate and use Serf. These details are documented here for those who wish
to learn about them without having to go spelunking through the source code.
</div>

## SWIM Protocol Overview

Serf begins by joining an existing cluster or starting a new
cluster. If starting a new cluster, additional nodes are expected to join
it. New nodes in an existing cluster must be given the address of at
least one existing member in order to join the cluster. The new member
does a full state sync with the existing member over TCP and begins gossiping its
existence to the cluster.

Gossip is done over UDP with a configurable but fixed fanout and interval.
This ensures that network usage is constant with regards to number of nodes.
Complete state exchanges with a random node are done periodically over
TCP, but much less often than gossip messages. This increases the likelihood
that the membership list converges properly since the full state is exchanged
and merged. The interval between full state exchanges is configurable or can
be disabled entirely.

Failure detection is done by periodic random probing using a configurable interval.
If the node fails to ack within a reasonable time (typically some multiple
of RTT), then an indirect probe is attempted. An indirect probe asks a
configurable number of random nodes to probe the same node, in case there
are network issues causing our own node to fail the probe. If both our
probe and the indirect probes fail within a reasonable time, then the
node is marked "suspicious" and this knowledge is gossiped to the cluster.
A suspicious node is still considered a member of cluster. If the suspect member
of the cluster does not dispute the suspicion within a configurable period of
time, the node is finally considered dead, and this state is then gossiped
to the cluster.

This is a brief and incomplete description of the protocol. For a better idea,
please read the
[SWIM paper](http://www.cs.cornell.edu/~asdas/research/dsn02-swim.pdf)
in its entirety, along with the Serf source code.

## SWIM Modifications

As mentioned earlier, the gossip protocol is based on SWIM but includes
minor changes, mostly to increase propogation speed and convergence rates.

The changes from SWIM are noted here:

* Serf does a full state sync over TCP periodically. SWIM only propagates
  changes over gossip. While both are eventually consistent, Serf is able to
  more quickly reach convergence, as well as gracefully recover from network
  partitions.

* Serf has a dedicated gossip layer separate from the failure detection
  protocol. SWIM only piggybacks gossip messages on top of probe/ack messages.
  Serf uses piggybacking along with dedicated gossip messages. This
  feature lets you have a higher gossip rate (for example once per 200ms)
  and a slower failure detection rate (such as once per second), resulting
  in overall faster convergence rates and data propagation speeds.

* Serf keeps the state of dead nodes around for a set amount of time,
  so that when full syncs are requested, the requester also receives information
  about dead nodes. Because SWIM doesn't do full syncs, SWIM deletes dead node
  state immediately upon learning that the node is dead. This change again helps
  the cluster converge more quickly.

## Serf-Specific Messages

On top of the SWIM-based gossip layer, Serf sends some custom message types.

Serf makes heavy use of [lamport clocks](http://en.wikipedia.org/wiki/Lamport_timestamps)
to maintain some notion of message ordering despite being eventually
consistent. Every message sent by Serf contains a lamport clock time.

When a node gracefully leaves the cluster, Serf sends a _leave intent_ through
the gossip layer. Because the underlying gossip layer makes no differentiation
between a node leaving the cluster and a node being detected as failed, this
allows the higher level Serf layer to detect a failure versus a graceful
leave.

When a node joins the cluster, Serf sends a _join intent_. The purpose
of this intent is solely to attach a lamport clock time to a join so that
it can be ordered properly in case a leave comes out of order.

For custom events, Serf sends a _user event_ message. This message contains
a lamport time, event name, and event payload. Because user events are sent
along the gossip layer, which uses UDP, the payload and entire message framing
must fit within a single UDP packet.
