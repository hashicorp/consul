import Component from '@ember/component';
import { set } from '@ember/object';
import Slotted from 'block-slots';
import chart from './chart.xstate';

export default Component.extend(Slotted, {
  tagName: '',
  ondelete: function() {
    return this.onchange(...arguments);
  },
  onchange: function() {},
  init: function() {
    this._super(...arguments);
    this.chart = chart;
  },
  actions: {
    persist: function(data, e) {
      if (e && typeof e.preventDefault === 'function') {
        e.preventDefault();
      }
      set(this, 'data', data);
      this.dispatch('PERSIST');
    },
    error: function(data, e) {
      if (e && typeof e.preventDefault === 'function') {
        e.preventDefault();
      }
      set(
        this,
        'error',
        typeof data.error.errors !== 'undefined' ?
          data.error.errors.firstObject : data.error
      );
      this.dispatch('ERROR');
    },
  },
});
