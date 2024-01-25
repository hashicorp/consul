import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

/**
 * If the user has accessed consul from HCP managed consul, we do NOT want to display the
 * "HCP Consul Central↗️" link in the nav bar. As we're already displaying a BackLink to HCP.
 */
export default class HcpLinkItemComponent extends Component {
  @service env;
  @service('hcp-link-status') hcpLinkStatus;

  get hcpLink() {
    // TODO: How do I figure this out? From the linking API?
    return 'https://corn.com';
  }

  get shouldDisplayNavLinkItem() {
    return this.hcpLinkStatus.hasPermissionToLink;
  }

  get shouldShowBackToHcpItem() {
    const isConsulHcpUrlDefined = !!this.env.var('CONSUL_HCP_URL');
    const isConsulHcpEnabled = !!this.env.var('CONSUL_HCP_ENABLED');
    return isConsulHcpEnabled && isConsulHcpUrlDefined;
  }

  get shouldDisplayNewBadge() {
    // TODO: Need a better name for this property
    return this.hcpLinkStatus.shouldDisplayBanner;
  }

  @action
  onLinkToConsulCentral() {
    // TODO: https://hashicorp.atlassian.net/browse/CC-7147 open the modal
  }
}
