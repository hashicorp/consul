import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class CollapsibleNotices extends Component {
  // =attributes
  @tracked collapsible = false;

  @action
  countNotices() {
    const allNotices = [
      ...document.querySelectorAll('div.collapsible-notices > div.notices > .notice'),
    ];
    this.collapsible = allNotices.length > 2;
  }
}
