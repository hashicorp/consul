import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class TopoloyMetricsButton extends Component {
  // =attributes
  @tracked showToggleablePopover = false;

  // =actions
  @action
  togglePopover() {
    this.showToggleablePopover = !this.showToggleablePopover;
  }
}
