import Component from '@glimmer/component';

const BADGE_LOOKUP = {
  ACTIVE: {
    bg: 'bg-[#F2FBF6]',
    color: 'text-[#00781E]',
  },
  INITIAL: {
    bg: 'bg-[#F9F2FF]',
    color: 'text-[#911CED]',
  },
  FAILING: {
    bg: 'bg-[#FFF5F5]',
    color: 'text-[#C00005]',
  },
  TERMINATED: {
    bg: 'bg-[#F1F2F3]',
    color: 'text-[#3B3D45]',
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
}
