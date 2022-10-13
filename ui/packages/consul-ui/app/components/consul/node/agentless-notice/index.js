import Component from '@glimmer/component';
import { action } from '@ember/object';
import { trackedInLocalStorage } from 'ember-tracked-local-storage';

export default class AgentlessNotice extends Component {
  @trackedInLocalStorage({ defaulValue: 'false' }) consulNodesAgentlessNoticeDismissed;

  get isVisible() {
    const { items, filteredItems } = this.args;

    return (
      this.consulNodesAgentlessNoticeDismissed !== 'true' && items.length > filteredItems.length
    );
  }

  @action
  dismissAgentlessNotice() {
    this.consulNodesAgentlessNoticeDismissed = 'true';
  }
}
