import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

const DISMISSED_VALUE = 'true';

export default class AgentlessNotice extends Component {
  storageKey = 'consul-nodes-agentless-notice-dismissed';
  @tracked hasDismissedNotice = false;

  constructor(owner, args) {
    super(owner, args);

    if (this.args.dc) {
      this.storageKey = `consul-nodes-agentless-notice-dismissed-${this.args.dc}`;
    }

    if (window.localStorage.getItem(this.storageKey) === DISMISSED_VALUE) {
      this.hasDismissedNotice = true;
    }
  }

  get isVisible() {
    const { items, filteredItems } = this.args;

    return !this.hasDismissedNotice && items.length > filteredItems.length;
  }

  @action
  dismissAgentlessNotice() {
    window.localStorage.setItem(this.storageKey, DISMISSED_VALUE);
    this.hasDismissedNotice = true;
  }
}
