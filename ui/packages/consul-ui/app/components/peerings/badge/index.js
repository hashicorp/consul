import Component from '@glimmer/component';

const BADGE_LOOKUP = {
  ACTIVE: {
    tooltip: 'This peer connection is currently active.',
  },
  PENDING: {
    tooltip: 'This peering connection has not been established yet.',
  },
  ESTABLISHING: {
    tooltip: 'This peering connection is in the process of being established.',
  },
  FAILING: {
    tooltip:
      'This peering connection has some intermittent errors (usually network related). It will continue to retry. ',
  },
  DELETING: {
    tooltip: 'This peer is in the process of being deleted.',
  },
  TERMINATED: {
    tooltip: 'Someone in the other peer may have deleted this peering connection.',
  },
  UNDEFINED: {
    tooltip: '',
  },
};
export default class PeeringsBadge extends Component {
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
