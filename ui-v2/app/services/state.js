import Service, { inject as service } from '@ember/service';
import { set } from '@ember/object';
import flat from 'flat';
import { createMachine, interpret } from '@xstate/fsm';

export default Service.extend({
  logger: service('logger'),
  // @xstate/fsm
  log: function(chart, state) {
    this.logger.execute(`${chart.id} > ${state.value}`);
  },
  addGuards: function(chart, options) {
    this.guards(chart).forEach(function([path, name]) {
      // xstate/fsm has no guard lookup
      set(chart, path, function() {
        return !!options.onGuard(...[name, ...arguments]);
      });
    });
    return [chart, options];
  },
  machine: function(chart, options = {}) {
    return createMachine(...this.addGuards(chart, options));
  },
  prepareChart: function(chart) {
    // xstate/fsm has no guard lookup so we clone the chart here
    // for when we replace the string based guards with functions
    // further down
    chart = JSON.parse(JSON.stringify(chart));
    // xstate/fsm doesn't seem to interpret toplevel/global events
    // artificially add them here instead
    if (typeof chart.on !== 'undefined') {
      Object.values(chart.states).forEach(function(state) {
        if (typeof state.on === 'undefined') {
          state.on = chart.on;
        } else {
          Object.keys(chart.on).forEach(function(key) {
            if (typeof state.on[key] === 'undefined') {
              state.on[key] = chart.on[key];
            }
          });
        }
      });
    }
    return chart;
  },
  // abstract
  matches: function(state, matches) {
    if (typeof state === 'undefined') {
      return false;
    }
    const values = Array.isArray(matches) ? matches : [matches];
    return values.some(item => {
      return state.matches(item);
    });
  },
  state: function(cb) {
    return {
      matches: cb,
    };
  },
  interpret: function(chart, options) {
    chart = this.prepareChart(chart);
    const service = interpret(this.machine(chart, options));
    // returns subscription
    service.subscribe(state => {
      if (state.changed) {
        this.log(chart, state);
        options.onTransition(state);
      }
    });
    return service;
  },
  guards: function(chart) {
    return Object.entries(flat(chart)).filter(([key]) => key.endsWith('.cond'));
  },
});
