import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class Certificate extends Component {
  // =attributes
  @tracked show = false;

  // =actions
  @action
  setVisibility() {
    this.show = !this.show;
  }
}
