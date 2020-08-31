import Component from '@ember/component';
import { inject as service } from '@ember/service';
import chart from './chart.xstate';

export default Component.extend({
  clipboard: service('clipboard/os'),
  dom: service('dom'),
  tagName: '',
  init: function() {
    this._super(...arguments);
    this.chart = chart;
    this.guid = this.dom.guid(this);
    this._listeners = this.dom.listeners();
  },
  willDestroyElement: function() {
    this._super(...arguments);
    this._listeners.remove();
  },
  didInsertElement: function() {
    this._super(...arguments);
    this._listeners.add(this.clipboard.execute(`#${this.guid} button`), {
      success: () => this.dispatch('SUCCESS'),
      error: () => this.dispatch('ERROR'),
    });
  },
});
