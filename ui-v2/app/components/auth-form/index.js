import Component from '@ember/component';

import chart from './chart.xstate';

export default Component.extend({
  tagName: '',
  onsubmit: function(e) {},
  onchange: function(e) {},
  init: function() {
    this._super(...arguments);
    this.chart = chart;
  },
  actions: {
    hasValue: function(context, event, meta) {
      return this.value !== '' && typeof this.value !== 'undefined';
    },
  },
});
