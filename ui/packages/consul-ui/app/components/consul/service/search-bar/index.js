import Component from '@glimmer/component';

export default class ConsulServiceSearchBar extends Component {
  get healthStates() {
    let states = ['passing', 'warning', 'critical', 'empty'];

    if (this.args.peer) {
      states = [...states, 'unknown'];
    }

    return states;
  }
}
