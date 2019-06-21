import Component from '@ember/component';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';

import WithListeners from 'consul-ui/mixins/with-listeners';

export default Component.extend(WithListeners, {
  clipboard: service('clipboard/os'),
  tagName: 'button',
  classNames: ['copy-btn'],
  buttonType: 'button',
  disabled: false,
  error: function() {},
  success: function() {},
  attributeBindings: [
    'clipboardText:data-clipboard-text',
    'clipboardTarget:data-clipboard-target',
    'clipboardAction:data-clipboard-action',
    'buttonType:type',
    'disabled',
    'aria-label',
    'title',
  ],
  delegateClickEvent: true,

  didInsertElement: function() {
    this._super(...arguments);
    const clipboard = get(this, 'clipboard').execute(
      this.delegateClickEvent ? `#${this.elementId}` : this.element
    );
    ['success', 'error'].map(event => {
      return this.listen(clipboard, event, () => {
        if (!this.disabled) {
          this[event](...arguments);
        }
      });
    });
  },
});
