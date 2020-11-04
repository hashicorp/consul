import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';

export default class TopoloyMetricsButton extends Component {
  // =services
  @service('repository/intention') repo;

  // =attributes
  @tracked showToggleablePopover = false;

  // =actions
  @action
  togglePopover() {
    this.showToggleablePopover = !this.showToggleablePopover;
  }
  @action
  open() {
    console.log('hello');
  }
  @action
  createIntention(obj) {
    this.repo.persist(obj);
  }
}
