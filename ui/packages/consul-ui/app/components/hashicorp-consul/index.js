import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class HashiCorpConsul extends Component {
  // TODO: Right now this is the only place where we need permissions
  // but we are likely to need it elsewhere, so probably need a nice helper
  get canManageNspaces() {
    return (
      typeof (this.args.permissions || []).find(function(item) {
        return item.Resource === 'operator' && item.Access === 'write' && item.Allow;
      }) !== 'undefined'
    );
  }

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
