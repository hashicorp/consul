import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';
import Component from 'consul-ui/components/dom-buffer';
import SlotsMixin from 'ember-block-slots';
import WithResizing from 'consul-ui/mixins/with-resizing';

import templatize from 'consul-ui/utils/templatize';
export default Component.extend(SlotsMixin, WithResizing, {
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
  didAppear: function() {
    this._super(...arguments);
    if (get(this, 'checked')) {
      get(this, 'dom')
        .root()
        .classList.add(...templatize(['with-modal']));
    }
  },
  _close: function(e) {
    set(this, 'checked', false);
    const dialogPanel = get(this, 'dialog');
    const overflowing = get(this, 'overflowingClass');
    if (dialogPanel.classList.contains(overflowing)) {
      dialogPanel.classList.remove(overflowing);
    }
    // TODO: should we make a didDisappear?
    get(this, 'dom')
      .root()
      .classList.remove(...templatize(['with-modal']));
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
    if (get(this, 'name')) {
      set(this, 'checked', false);
    }
    if (this.element) {
      if (get(this, 'checked')) {
        // TODO: probably need an event here
        // possibly this.element for the target
        // or find the input
        this._open({ target: {} });
      }
    }
  },
  didInsertElement: function() {
    this._super(...arguments);
    if (get(this, 'checked')) {
      // TODO: probably need an event here
      // possibly this.element for the target
      // or find the input
      this._open({ target: {} });
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
