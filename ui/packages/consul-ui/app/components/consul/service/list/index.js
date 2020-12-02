import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class ConsulServiceList extends Component {
  @action
  isLinkable(item) {
    return item.InstanceCount > 0;
  }
}
