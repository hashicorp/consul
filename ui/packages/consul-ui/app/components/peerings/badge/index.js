import Component from '@glimmer/component';

const BADGE_LOOKUP = {
  ACTIVE: {},
  INITIAL: {
    tooltip:
      'A token has been generated for this peer, but has not yet been initiated in the remote peer.',
  },
  FAILING: {
    tooltip:
      'This peering connection has some intermittent errors (usually network related). It will continue to retry. ',
  },
  TERMINATED: {
    tooltip: 'Someone in the other peer may have deleted this peering connection.',
  },
  UNDEFINED: {},
};
export default class PeeingsBadge extends Component {
  get styles() {
    const {
      peering: { State },
    } = this.args;

    return BADGE_LOOKUP[State];
  }

  get tooltip() {
    return this.styles.tooltip;
  }
}
