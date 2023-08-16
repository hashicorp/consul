/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set } from '@ember/object';

export default Component.extend({
  chart: service('state'),
  tagName: '',
  ontransition: function (e) {},
  init: function () {
    this._super(...arguments);
    this._actions = {};
    this._guards = {};
  },
  didReceiveAttrs: function () {
    if (typeof this.machine !== 'undefined') {
      this.machine.stop();
    }
    if (typeof this.initial !== 'undefined') {
      this.src.initial = this.initial;
    }
    this.machine = this.chart.interpret(this.src, {
      onTransition: (state) => {
        const e = new CustomEvent('transition', { detail: state });
        this.ontransition(e);
        if (!e.defaultPrevented) {
          state.actions.forEach((item) => {
            const action = this._actions[item.type];
            if (typeof action === 'function') {
              this._actions[item.type](item.type, state.context, state.event);
            }
          });
        }
        set(this, 'state', state);
      },
      onGuard: (name, ...rest) => {
        return this._guards[name](...rest);
      },
    });
  },
  didInsertElement: function () {
    this._super(...arguments);
    // xstate has initialState xstate/fsm has state
    set(this, 'state', this.machine.initialState || this.machine.state);
    // set(this, 'state', this.machine.initialState);
    this.machine.start();
  },
  willDestroy: function () {
    this._super(...arguments);
    this.machine.stop();
  },
  addAction: function (name, value) {
    this._actions[name] = value;
  },
  removeAction: function (name) {
    delete this._actions[name];
  },
  addGuard: function (name, value) {
    this._guards[name] = value;
  },
  removeGuard: function (name) {
    delete this._guards[name];
  },
  dispatch: function (eventName, payload) {
    this.machine.state.context = payload;
    this.machine.send({ type: eventName });
  },
  actions: {
    dispatch: function (eventName, e) {
      if (e && e.preventDefault) {
        if (typeof e.target.nodeName === 'undefined' || e.target.nodeName.toLowerCase() !== 'a') {
          e.preventDefault();
        }
      }
      this.dispatch(eventName, e);
    },
  },
});
