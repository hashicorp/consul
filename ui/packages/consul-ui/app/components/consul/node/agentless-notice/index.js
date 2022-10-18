import Component from '@glimmer/component';
import { action } from '@ember/object';
import { storageFor } from '../../../../services/local-storage';

export default class AgentlessNotice extends Component {
  storageKey = 'nodes-agentless-dismissed';
  @storageFor('notices') notices;

  constructor(owner, args) {
    super(owner, args);

    if (this.args.postfix) {
      this.storageKey = `nodes-agentless-dismissed-${this.args.postfix}`;
    }
  }

  get isVisible() {
    const { items, filteredItems } = this.args;

    return !this.notices.state.includes(this.storageKey) && items.length > filteredItems.length;
  }

  @action
  dismissAgentlessNotice() {
    this.notices.add(this.storageKey);
  }
}
