import Component from '@ember/component';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';
import Slotted from 'block-slots';
import A11yDialog from 'a11y-dialog'

export default Component.extend(Slotted, {
  tagName: '',
  onclose: function() {},
  onopen: function() {},
  actions: {
    connect: function($el) {
      this.dialog = new A11yDialog($el);
      this.dialog.on(
        'hide',
        () => this.onclose()
      );
      this.dialog.on(
        'show',
        () => this.onopen()
      );
    },
    disconnect: function($el) {
      this.dialog.destroy();
    },
    open: function() {
      this.dialog.show();
    },
    close: function() {
      this.dialog.hide();
    },
    change: function(e) {
      this.actions.open.call(this);
    },
  },
});
