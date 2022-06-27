import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class PeeringsServiceCount extends Component {
  @service intl;

  get count() {
    const { peering, kind } = this.args;

    return peering[`${kind.capitalize()}ServiceCount`];
  }

  get text() {
    const { kind } = this.args;
    const { intl, count } = this;

    return intl.t(`routes.dc.peers.index.detail.${kind}.count`, { count });
  }

  get tooltipText() {
    const {
      kind,
      peering: { name },
    } = this.args;
    const { intl } = this;

    return intl.t(`routes.dc.peers.index.detail.${kind}.tooltip`, { name });
  }
}
