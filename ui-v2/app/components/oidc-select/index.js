import Component from '@ember/component';
import chart from './chart.xstate';

export default Component.extend({
  tagName: '',
  onchange: function() {},
  onerror: function() {},
  init: function() {
    this._super(...arguments);
    this.chart = chart;
  },
  didReceiveAttrs: function() {
    // This gets called no matter which attr has changed
    // such as disabled, thing is once we are in loaded state
    // it doesn't do anything anymore
    if (typeof this.items !== 'undefined') {
      this.dispatch('SUCCESS');
    }
  },
});
