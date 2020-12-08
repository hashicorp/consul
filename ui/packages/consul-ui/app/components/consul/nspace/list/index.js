import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class ConsulNspaceList extends Component {
  isLinkable(item) {
    return !item.DeletedAt;
  }
}
