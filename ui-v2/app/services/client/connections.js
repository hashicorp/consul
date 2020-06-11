import Service, { inject as service } from '@ember/service';
import { set } from '@ember/object';

import getObjectPool from 'consul-ui/utils/get-object-pool';

const dispose = function(obj) {
  if (typeof obj.dispose === 'function') {
    obj.dispose();
  }
  return obj;
};

export default Service.extend({
  dom: service('dom'),
  env: service('env'),
  init: function() {
    this._super(...arguments);
    this._listeners = this.dom.listeners();
    set(this, 'connections', getObjectPool(dispose, this.env.var('CONSUL_HTTP_MAX_CONNECTIONS')));
    this.addVisibilityChange();
  },
  willDestroy: function() {
    this._listeners.remove();
    this.connections.purge();
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
    // if we are using a connection limited protocol and the user has hidden the tab (hidden browser/tab switch)
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
  // We overwrite this for testing purposes in /tests/test-helper.js
  // therefore until we reorganize the client into sub-services
  // any changes here will need to also be made in /tests/test-helper.js
  purge: function() {
    return this.connections.purge(...arguments);
  },
  acquire: function() {
    return this.connections.acquire(...arguments);
  },
  release: function() {
    return this.connections.release(...arguments);
  },
});
