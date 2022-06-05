import Component from '@glimmer/component';

const BADGE_LOOKUP = {
  ACTIVE: {
    bg: 'bg-[#F2FBF6]',
    color: 'text-[#00781E]',
  },
  INITIAL: {
    bg: 'bg-[#F9F2FF]',
    color: 'text-[#911CED]',
    tooltip:
      'A token has been generated for this peer, but has not yet been initiated in the remote peer.',
  },
  FAILING: {
    bg: 'bg-[#FFF5F5]',
    color: 'text-[#C00005]',
    tooltip:
      'This peering connection has some intermittent errors (usually network related). It will continue to retry. ',
  },
  TERMINATED: {
    bg: 'bg-[#F1F2F3]',
    color: 'text-[#3B3D45]',
    tooltip:
      'Someone in the other peer may have deleted this peering connection.',
  },
};
export default class PeeingsBadge extends Component {
  get styles() {
    const {
      peering: { state },
    } = this.args;

    return BADGE_LOOKUP[state];
  }

  get background() {
    return this.styles.bg;
  }

  get color() {
    return this.styles.color;
  }

  get tooltip() {
    return this.styles.tooltip;
  }
}
