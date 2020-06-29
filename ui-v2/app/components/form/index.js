import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set, get } from '@ember/object';
import Slotted from 'block-slots';
import chart from './chart.xstate';

export default Component.extend(Slotted, {
  tagName: '',
  dom: service('dom'),
  builder: service('form'),
  create: false,
  ondelete: function() {
    return this.onsubmit(...arguments);
  },
  oncancel: function() {
    return this.onsubmit(...arguments);
  },
  onsubmit: function() {},
  onchange: function(e, form) {
    return form.handleEvent(e);
  },
  init: function() {
    this._super(...arguments);
    this.chart = chart;
  },
  didReceiveAttrs: function() {
    this._super(...arguments);
    try {
      this.form = this.builder.form(this.type);
    } catch (e) {}
    if (typeof this.item !== 'undefined') {
      this.actions.setData.apply(this, [this.item]);
    }
  },
  didInsertElement: function() {
    this._super(...arguments);
    this.dispatch('LOAD');
  },
  willDestroyElement: function() {
    this._super(...arguments);
    if (get(this, 'data.isNew')) {
      this.data.rollbackAttributes();
    }
  },
  actions: {
    setData: function(data) {
      let changeset = data;
      if (typeof this.form !== 'undefined') {
        changeset = this.form.setData(data).getData();
      }
      if (get(changeset, 'isNew')) {
        set(this, 'create', true);
        changeset = Object.entries(this.autofill || {}).reduce(function(prev, [key, value]) {
          set(prev, key, value);
          return prev;
        }, changeset);
      }
      set(this, 'data', changeset);
    },
    isLoaded: function() {
      return typeof this.item !== 'undefined';
    },
    change: function(e, value, item) {
      this.onchange(this.dom.normalizeEvent(e, value), this.form, this.form.getData());
    },
  },
});
