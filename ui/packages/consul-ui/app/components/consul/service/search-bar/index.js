import Component from '@glimmer/component';

export default class ConsulServiceSearchBar extends Component {
  get healthStates() {
    if (this.args.peer) {
      return ['passing', 'warning', 'critical', 'unknown', 'empty'];
    } else {
      return ['passing', 'warning', 'critical', 'empty'];
    }
  }

  get sortedSources() {
    const sources = this.args.sources || [];

    if (sources.includes('consul-api-gateway')) {
      return [...sources.filter((s) => s !== 'consul-api-gateway'), 'consul-api-gateway'];
    } else {
      return sources;
    }
  }
}
