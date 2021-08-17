import Component from '@glimmer/component';
import { get } from '@ember/object';

export default class TopologyMetrics extends Component {
  // =methods
  get hrefPath() {
    const source = get(this.args.item, 'Source') || '';

    return source === 'routing-config' ? 'dc.routing-config' : 'dc.services.show.index';
  }
}
