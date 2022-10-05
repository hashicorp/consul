import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { hrefTo } from 'consul-ui/helpers/href-to';
import { inject as service } from '@ember/service';
import { getOwner } from '@ember/application';

class PeerTab {
  @tracked route;
  @tracked label;
  @tracked currentRouteName;

  constructor(opts) {
    const { currentRouteName, route, label, owner } = opts;

    this.currentRouteName = currentRouteName;
    this.owner = owner;
    this.route = route;
    this.label = label;
  }

  get selected() {
    return this.currentRouteName === this.route;
  }

  get href() {
    return hrefTo(this.owner, [this.route]);
  }
}

export default class PeeringsProvider extends Component {
  @service router;

  get data() {
    return {
      tabs: this.tabs,
    };
  }

  get tabs() {
    const { peer } = this.args;
    const { router } = this;
    const owner = getOwner(this);

    let tabs;
    if (peer.isDialer) {
      tabs = [
        {
          label: 'Exported Services',
          route: 'dc.peers.edit.exported',
        },
      ];
    } else {
      tabs = [
        { label: 'Imported Services', route: 'dc.peers.edit.imported' },
        { label: 'Addresses', route: 'dc.peers.edit.addresses' },
      ];
    }

    return tabs.map(
      (tab) => new PeerTab({ ...tab, currentRouteName: router.currentRouteName, owner })
    );
  }
}
