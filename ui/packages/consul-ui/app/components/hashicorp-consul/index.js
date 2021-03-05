import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class HashiCorpConsul extends Component {
  @action
  open() {
    this.authForm.focus();
  }

  @action
  close() {
    this.authForm.reset();
  }

  @action
  reauthorize(e) {
    this.modal.close();
    this.args.onchange(e);
  }

  @action
  keypressClick(e) {
    e.target.dispatchEvent(new MouseEvent('click'));
  }
}
