import Component from '@glimmer/component';

export default class ConsulNspaceList extends Component {
  isLinkable(item) {
    return !item.DeletedAt;
  }
}
