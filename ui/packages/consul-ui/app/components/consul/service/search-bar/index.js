import Component from '@glimmer/component';

export default class ConsulServiceSearchBar extends Component {
  get healthStates() {
    if (this.args.peer) {
      return ['passing', 'warning', 'critical', 'unknown', 'empty'];
    } else {
      return ['passing', 'warning', 'critical', 'empty'];
    }
  }
}
