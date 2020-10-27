import Component from '@ember/component';
import { computed } from '@ember/object';
import Ember from 'ember';

import chart from './chart.xstate';

export default Component.extend({
  tagName: '',
  onsubmit: function(e) {},
  onchange: function(e) {},
  // Blink/Webkit based seem to leak password inputs
  // this will only occur during acceptance testing so
  // turn them into text inputs during acceptance testing
  inputType: computed(function() {
    return Ember.testing ? 'text' : 'password';
  }),
  init: function() {
    this._super(...arguments);
    this.chart = chart;
  },
  actions: {
    hasValue: function(context, event, meta) {
      return this.value !== '' && typeof this.value !== 'undefined';
    },
    focus: function() {
      this.input.focus();
    },
  },
});
