/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service, { inject as service } from '@ember/service';

export default class ConnectionsService extends Service {
  @service('dom')
  dom;

  @service('env')
  env;

  @service('data-source/service')
  data;

  init() {
    super.init(...arguments);
    this._listeners = this.dom.listeners();
    this.connections = new Set();
    this.addVisibilityChange();
  }

  willDestroy() {
    this._listeners.remove();
    this.purge();
    super.willDestroy(...arguments);
  }

  addVisibilityChange() {
    // when the user hides the tab, abort all connections
    this._listeners.add(this.dom.document(), {
      visibilitychange: (e) => {
        if (e.target.hidden) {
          this.purge(-1);
        }
      },
    });
  }

  whenAvailable(e) {
    // if the user has hidden the tab (hidden browser/tab switch)
    // any aborted errors should restart
    const doc = this.dom.document();
    if (doc.hidden) {
      return new Promise((resolve) => {
        const remove = this._listeners.add(doc, {
          visibilitychange: function (event) {
            remove();
            // we resolve with the event that comes from
            // whenAvailable not visibilitychange
            resolve(e);
          },
        });
      });
    }
    return Promise.resolve(e);
  }

  purge(statusCode = 0) {
    [...this.connections].forEach(function (connection) {
      // Cancelled
      connection.abort(statusCode);
    });
    this.connections = new Set();
  }

  acquire(request) {
    if (this.connections.size >= this.env.var('CONSUL_HTTP_MAX_CONNECTIONS')) {
      const closed = this.data.closed();
      let connection = [...this.connections].find((item) => {
        const id = item.headers()['x-request-id'];
        if (id) {
          return closed.includes(item.headers()['x-request-id']);
        }
        return false;
      });
      if (typeof connection === 'undefined') {
        // all connections are being used on the page
        // if the new one is a blocking query then cancel the oldest connection
        if (request.headers()['content-type'] === 'text/event-stream') {
          connection = this.connections.values().next().value;
        }
        // otherwise wait for a connection to become available
      }
      // cancel the connection
      if (typeof connection !== 'undefined') {
        // if its a shared blocking query cancel everything
        // listening to it
        this.release(connection);
        // Too Many Requests
        connection.abort(429);
      }
    }
    this.connections.add(request);
  }

  release(request) {
    this.connections.delete(request);
  }
}
