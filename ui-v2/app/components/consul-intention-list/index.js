import Component from '@ember/component';

import chart from './chart.xstate';
export default Component.extend({
  tagName: '',
  ondelete: function() {},
  init: function() {
    this._super(...arguments);
    this.chart = chart;
  },
});
