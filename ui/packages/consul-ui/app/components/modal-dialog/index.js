import Component from '@ember/component';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';
import Slotted from 'block-slots';

import templatize from 'consul-ui/utils/templatize';

export default Component.extend(Slotted, {
  tagName: '',
  dom: service('dom'),
  checked: true,
  height: null,
  // dialog is a reference to the modal-dialog 'panel' so its 'window'
  dialog: null,
  overflowingClass: 'overflowing',
  onclose: function() {},
  onopen: function() {},
  _open: function(e) {
    set(this, 'checked', true);
    if (this.height === null) {
      if (this.element) {
        const dialogPanel = this.dom.element('[role="dialog"] > div > div', this.modal);
        const rect = dialogPanel.getBoundingClientRect();
        set(this, 'dialog', dialogPanel);
        set(this, 'height', rect.height);
      }
    }
    this.didAppear();
    this.onopen(e);
  },
  didAppear: function() {
    this._super(...arguments);
    if (this.checked) {
      this.dom.root().classList.add(...templatize(['with-modal']));
    }
  },
  _close: function(e) {
    set(this, 'checked', false);
    const dialogPanel = this.dialog;
    if (dialogPanel) {
      const overflowing = this.overflowingClass;
      if (dialogPanel.classList.contains(overflowing)) {
        dialogPanel.classList.remove(overflowing);
      }
    }
    // TODO: should we make a didDisappear?
    this.dom.root().classList.remove(...templatize(['with-modal']));
    this.onclose(e);
  },
  didReceiveAttrs: function() {
    this._super(...arguments);
    // TODO: Why does setting name mean checked it false?
    // It's because if it has a name then it is likely to be linked
    // to HTML state rather than just being added via HTMLBars
    // and therefore likely to be immediately on the page
    // It's not our usecase just yet, but this should check the state
    // of the thing its linked to, incase that has a `checked` of true
    // right now we know ours is always false.
    if (this.name) {
      set(this, 'checked', false);
    }
    if (this.element) {
      if (this.checked) {
        // TODO: probably need an event here
        // possibly this.element for the target
        // or find the input
        this._open({ target: {} });
      }
    }
  },
  didInsertElement: function() {
    this._super(...arguments);
    this.actions.resize.apply(this, [{ target: this.dom.viewport() }]);
    if (this.checked) {
      // TODO: probably need an event here
      // possibly this.element for the target
      // or find the input
      this._open({ target: {} });
    }
  },
  didDestroyElement: function() {
    this._super(...arguments);
    this.dom.root().classList.remove(...templatize(['with-modal']));
  },
  actions: {
    resize: function(e) {
      if (this.checked) {
        const height = this.height;
        if (height !== null) {
          const dialogPanel = this.dialog;
          const overflowing = this.overflowingClass;
          if (height > e.target.innerHeight) {
            if (!dialogPanel.classList.contains(overflowing)) {
              dialogPanel.classList.add(overflowing);
            }
            return;
          } else {
            if (dialogPanel.classList.contains(overflowing)) {
              dialogPanel.classList.remove(overflowing);
            }
          }
        }
      }
    },
    change: function(e) {
      if (get(e, 'target.checked')) {
        this._open(e);
      } else {
        this._close(e);
      }
    },
    close: function() {
      const $close = this.dom.element('#modal_close');
      $close.checked = true;
      const $input = this.dom.element('input[name="modal"]', this.modal);
      $input.onchange({ target: $input });
    },
  },
});
