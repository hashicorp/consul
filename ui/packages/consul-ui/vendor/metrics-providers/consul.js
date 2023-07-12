(function(global) {
  // Current interface is these three methods.
  const requiredMethods = [
    'init',
    'serviceRecentSummarySeries',
    'serviceRecentSummaryStats',
    'upstreamRecentSummaryStats',
    'downstreamRecentSummaryStats',
  ];

  // This is a bit gross but we want to support simple extensibility by
  // including JS in the browser without forcing operators to setup a whole
  // transpiling stack. So for now we use a window global as a thin registry for
  // these providers.
  class Consul {
    constructor() {
      this.registry = {};
      this.providers = {};
    }

    registerMetricsProvider(name, provider) {
      // Basic check that it matches the type we expect
      for (var m of requiredMethods) {
        if (typeof provider[m] !== 'function') {
          throw new Error(`Can't register metrics provider '${name}': missing ${m} method.`);
        }
      }
      this.registry[name] = provider;
    }

    getMetricsProvider(name, options) {
      if (!(name in this.registry)) {
        throw new Error(`Metrics Provider '${name}' is not registered.`);
      }
      if (name in this.providers) {
        return this.providers[name];
      }

      this.providers[name] = Object.create(this.registry[name]);
      this.providers[name].init(options);

      return this.providers[name];
    }
  }

  global.consul = new Consul();
})(window);
