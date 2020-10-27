import Component from '@ember/component';
import { set } from '@ember/object';
import Slotted from 'block-slots';

import chart from './chart.xstate';
export default Component.extend(Slotted, {
  tagName: '',
  onchange: data => data,
  init: function() {
    this._super(...arguments);
    this.chart = chart;
  },
  didReceiveAttrs: function() {
    this._super(...arguments);
    if (typeof this.items !== 'undefined') {
      this.actions.change.apply(this, [this.items]);
    }
  },
  didInsertElement: function() {
    this._super(...arguments);
    this.dispatch('LOAD');
  },
  actions: {
    isLoaded: function() {
      return typeof this.items !== 'undefined' || typeof this.src === 'undefined';
    },
    change: function(data) {
      set(this, 'data', this.onchange(data));
    },
  },
});
