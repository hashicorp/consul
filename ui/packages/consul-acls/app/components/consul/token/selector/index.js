import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class ConsulAclsTokensSelector extends Component {
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
}
