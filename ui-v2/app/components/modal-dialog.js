import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';
import Component from 'consul-ui/components/dom-buffer';
import SlotsMixin from 'ember-block-slots';
import WithResizing from 'consul-ui/mixins/with-resizing';

import templatize from 'consul-ui/utils/templatize';
const $html = document.documentElement;
export default Component.extend(SlotsMixin, WithResizing, {
  // tagName: ''
  dom: service('dom'),
  checked: true,
  height: null,
  // WithResizing already uses `win`
  window: null,
  overflowingClass: 'overflowing',
  onclose: function() {},
  onopen: function() {},
  _open: function(e) {
    set(this, 'checked', true);
    if (get(this, 'height') === null) {
      if (this.element) {
        const win = [...get(this, 'dom').element('[role="dialog"] > div > div', this.element)][0];
        const rect = win.getBoundingClientRect();
        set(this, 'window', win);
        set(this, 'height', rect.height);
      }
    }
    this.onopen(e);
  },
  _close: function(e) {
    set(this, 'checked', false);
    const win = get(this, 'window');
    const overflowing = get(this, 'overflowingClass');
    if (win.classList.contains(overflowing)) {
      win.classList.remove(overflowing);
    }
    this.onclose(e);
  },
  didReceiveAttrs: function() {
    this._super(...arguments);
    if (get(this, 'name')) {
      set(this, 'checked', false);
    }
    if (get(this, 'checked')) {
      // TODO: probably need an event here
      this._open({ target: {} });
      $html.classList.add(...templatize(['with-modal']));
    }
  },
  didDestroyElement: function() {
    this._super(...arguments);
    $html.classList.remove(...templatize(['with-modal']));
  },
  resize: function(e) {
    if (get(this, 'checked')) {
      const height = get(this, 'height');
      if (height !== null) {
        const win = get(this, 'window');
        const overflowing = get(this, 'overflowingClass');
        if (height > e.detail.height) {
          if (!win.classList.contains(overflowing)) {
            win.classList.add(overflowing);
          }
          return;
        } else {
          if (win.classList.contains(overflowing)) {
            win.classList.remove(overflowing);
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
      document.getElementById('modal_close').checked = true;
      this.onclose();
    },
  },
});
