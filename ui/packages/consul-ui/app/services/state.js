/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Service, { inject as service } from '@ember/service';
import { set } from '@ember/object';
import flat from 'flat';
import { createMachine, interpret } from '@xstate/fsm';

export default class StateService extends Service {
  stateCharts = {};

  @service('logger') logger;

  // @xstate/fsm
  log(chart, state) {
    // this.logger.execute(`${chart.id} > ${state.value}`);
  }

  stateChart(name) {
    return this.stateCharts[name];
  }

  addGuards(chart, options) {
    this.guards(chart).forEach(function ([path, name]) {
      // xstate/fsm has no guard lookup
      set(chart, path, function () {
        return !!options.onGuard(...[name, ...arguments]);
      });
    });
    return [chart, options];
  }

  machine(chart, options = {}) {
    return createMachine(...this.addGuards(chart, options));
  }

  prepareChart(chart) {
    // xstate/fsm has no guard lookup so we clone the chart here
    // for when we replace the string based guards with functions
    // further down
    chart = JSON.parse(JSON.stringify(chart));
    // xstate/fsm doesn't seem to interpret toplevel/global events
    // artificially add them here instead
    if (typeof chart.on !== 'undefined') {
      Object.values(chart.states).forEach(function (state) {
        if (typeof state.on === 'undefined') {
          state.on = chart.on;
        } else {
          Object.keys(chart.on).forEach(function (key) {
            if (typeof state.on[key] === 'undefined') {
              state.on[key] = chart.on[key];
            }
          });
        }
      });
    }
    return chart;
  }

  // abstract
  matches(state, matches) {
    if (typeof state === 'undefined') {
      return false;
    }
    const values = Array.isArray(matches) ? matches : [matches];
    return values.some((item) => {
      return state.matches(item);
    });
  }

  state(cb) {
    return {
      matches: cb,
    };
  }

  interpret(chart, options) {
    chart = this.prepareChart(chart);
    const service = interpret(this.machine(chart, options));
    // returns subscription
    service.subscribe((state) => {
      if (state.changed) {
        this.log(chart, state);
        options.onTransition(state);
      }
    });
    return service;
  }

  guards(chart) {
    return Object.entries(flat(chart)).filter(([key]) => key.endsWith('.cond'));
  }
}
