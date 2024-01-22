import Service from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';

const LOCAL_STORAGE_KEY = 'consul:hideHcpLinkBanner';

export default class HcpLinkStatus extends Service {
  @service('ui-config') config;

  @tracked
  alreadyLinked = false;
  @tracked
  userDismissedBanner = false;
  @tracked
  cloudConfigDisabled = false;

  get shouldDisplayBanner() {
    return !this.alreadyLinked && !this.userDismissedBanner && !this.cloudConfigDisabled;
  }

  constructor() {
    super(...arguments);
    const config = this.config.getSync();
    this.cloudConfigDisabled = config?.cloud?.enabled === false;
    this.userDismissedBanner = !!localStorage.getItem(LOCAL_STORAGE_KEY);
  }

  userHasLinked() {
    // TODO: CC-7145 - once can fetch the link status from the backend, fetch it and set it here
  }

  dismissHcpLinkBanner() {
    localStorage.setItem(LOCAL_STORAGE_KEY, true);
    this.userDismissedBanner = true;
  }
}
