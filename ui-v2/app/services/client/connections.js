import Service, { inject as service } from '@ember/service';

export default Service.extend({
  dom: service('dom'),
  env: service('env'),
  init: function() {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
    this.connections = new Set();
    this.addVisibilityChange();
  },
  willDestroy: function() {
    this._listeners.remove();
    this.purge();
    this._super(...arguments);
  },
  addVisibilityChange: function() {
    // when the user hides the tab, abort all connections
    this._listeners.add(this.dom.document(), {
      visibilitychange: e => {
        if (e.target.hidden) {
          this.purge();
        }
      },
    });
  },
  whenAvailable: function(e) {
    // if the user has hidden the tab (hidden browser/tab switch)
    // any aborted errors should restart
    const doc = this.dom.document();
    if (doc.hidden) {
      return new Promise(resolve => {
        const remove = this._listeners.add(doc, {
          visibilitychange: function(event) {
            remove();
            // we resolve with the event that comes from
            // whenAvailable not visibilitychange
            resolve(e);
          },
        });
      });
    }
    return Promise.resolve(e);
  },
  purge: function() {
    [...this.connections].forEach(function(connection) {
      // Cancelled
      connection.abort(0);
    });
    this.connections = new Set();
  },
  acquire: function(request) {
    this.connections.add(request);
    if (this.connections.size > this.env.var('CONSUL_HTTP_MAX_CONNECTIONS')) {
      const connection = this.connections.values().next().value;
      this.connections.delete(connection);
      // Too Many Requests
      connection.abort(429);
    }
  },
  release: function(request) {
    this.connections.delete(request);
  },
});
