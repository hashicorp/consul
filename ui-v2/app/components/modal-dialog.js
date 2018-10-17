import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';
import Component from 'consul-ui/components/dom-buffer';
import SlotsMixin from 'ember-block-slots';
import WithResizing from 'consul-ui/mixins/with-resizing';

import templatize from 'consul-ui/utils/templatize';
export default Component.extend(SlotsMixin, WithResizing, {
  // tagName: ''
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
    if (get(this, 'height') === null) {
      if (this.element) {
        const dialogPanel = get(this, 'dom').element('[role="dialog"] > div > div', this.element);
        const rect = dialogPanel.getBoundingClientRect();
        set(this, 'dialog', dialogPanel);
        set(this, 'height', rect.height);
      }
    }
    this.didAppear();
    this.onopen(e);
  },
  _close: function(e) {
    set(this, 'checked', false);
    const dialogPanel = get(this, 'dialog');
    const overflowing = get(this, 'overflowingClass');
    if (dialogPanel.classList.contains(overflowing)) {
      dialogPanel.classList.remove(overflowing);
    }
    this.onclose(e);
  },
  didReceiveAttrs: function() {
    this._super(...arguments);
    // TODO: Why does setting name mean checked it false?
    if (get(this, 'name')) {
      set(this, 'checked', false);
    }
    if (get(this, 'checked')) {
      // TODO: probably need an event here
      this._open({ target: {} });
      get(this, 'dom')
        .root()
        .classList.add(...templatize(['with-modal']));
    }
  },
  didDestroyElement: function() {
    this._super(...arguments);
    get(this, 'dom')
      .root()
      .classList.remove(...templatize(['with-modal']));
  },
  resize: function(e) {
    if (get(this, 'checked')) {
      const height = get(this, 'height');
      if (height !== null) {
        const dialogPanel = get(this, 'dialog');
        const overflowing = get(this, 'overflowingClass');
        if (height > e.detail.height) {
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
  actions: {
    change: function(e) {
      if (e && e.target && e.target.checked) {
        this._open(e);
      } else {
        this._close();
      }
    },
    close: function() {
      get(this, 'dom').element('#modal_close').checked = true;
      this.onclose();
    },
  },
});
