import Component from '@ember/component';
import Slotted from 'block-slots';
import A11yDialog from 'a11y-dialog';

export default Component.extend(Slotted, {
  tagName: '',
  onclose: function() {},
  onopen: function() {},
  actions: {
    connect: function($el) {
      this.dialog = new A11yDialog($el);
      this.dialog.on('hide', () => this.onclose({ target: $el }));
      this.dialog.on('show', () => this.onopen({ target: $el }));
      if (this.open) {
        this.dialog.show();
      }
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
  },
});
