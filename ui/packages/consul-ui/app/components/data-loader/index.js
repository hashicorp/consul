/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { set } from '@ember/object';
import Slotted from 'block-slots';

import chart from './chart.xstate';
export default Component.extend(Slotted, {
  tagName: '',
  onchange: (data) => data,
  init: function () {
    this._super(...arguments);
    this.chart = chart;
  },
  didReceiveAttrs: function () {
    this._super(...arguments);
    if (typeof this.items !== 'undefined') {
      this.send('change', this.items);
    }
  },
  didInsertElement: function () {
    this._super(...arguments);
    this.dispatch('LOAD');
  },
  actions: {
    isLoaded: function () {
      return typeof this.items !== 'undefined' || typeof this.src === 'undefined';
    },
    // caching data for namesapce page only currently to avoid showing a Welcome screen when we switch tabs
    // and come back to this page.
    // For other pages, it will behave as before without a need of explicit caching logic used here.
    change: function (data) {
      if (
        this.useCachedOnEmpty && // Only apply for namespace page
        ((Array.isArray(data) && data.length === 0) || (!Array.isArray(data) && !data))
      ) {
        if (this._lastKnownData) {
          set(this, 'data', this.onchange(this._lastKnownData));
        } else {
          set(this, 'data', this.onchange(data));
        }
      } else {
        set(this, 'data', this.onchange(data));
        this._lastKnownData = data;
      }
    },
  },
});
