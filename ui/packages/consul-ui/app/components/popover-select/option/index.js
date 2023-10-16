import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class Option extends Component {
  @tracked selected;

  @action
  connect() {
    this.args.select.addOption(this);
  }
  @action
  disconnect() {
    this.args.select.removeOption(this);
  }
}
