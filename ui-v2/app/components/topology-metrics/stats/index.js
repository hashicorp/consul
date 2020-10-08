import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { env } from 'consul-ui/env';

// CONSUL_METRICS_LATENCY_MAX is a fake delay (in milliseconds) we can set in
// cookies to simulate metrics requests taking a while to see loading state.
// It's the maximum time to wait and we'll randomly pick a time between tha and
// half of it if set.
function fakeMetricsLatency() {
  const fakeLatencyMax = env("CONSUL_METRICS_LATENCY_MAX", 0);
  if (fakeLatencyMax == 0) {
    return 0
  }
  return Math.random() * (fakeLatencyMax/2) + (fakeLatencyMax/2);
}

export default class TopologyMetricsStats extends Component {
  @tracked stats = null;
  @tracked hasLoaded = false;

  @action
  statsUpdate(event) {
    setTimeout(()=>{
      if (this.args.endpoint == "summary-for-service") {
        // For the main service there is just one set of stats.
        this.stats = event.data.stats;
      } else {
        // For up/downstreams we need to pull out the stats for the service we
        // represent.
        this.stats = event.data.stats[this.args.item]
      }
      // Limit to 4 metrics for now.
      this.stats = (this.stats || []).slice(0,4)
      this.hasLoaded = true;
    },
    fakeMetricsLatency());
  }
}