import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class TopologyMetricsStats extends Component {
  @tracked stats = null;
  @tracked hasLoaded = false;

  @action
  statsUpdate(event) {
    if (this.args.endpoint == 'summary-for-service') {
      // For the main service there is just one set of stats.
      this.stats = event.data.stats;
    } else {
      // For up/downstreams we need to pull out the stats for the service we
      // represent.
      let nspace = this.args.nspace || '';
      if (nspace.length === 0) {
        nspace = 'default';
      }
      let entity = `${this.args.item}.${nspace}.${this.args.dc}`;
      this.stats = event.data.stats[entity];
    }
    // Limit to 4 metrics for now.
    this.stats = (this.stats || []).slice(0, 4);
    this.hasLoaded = true;
  }
}
