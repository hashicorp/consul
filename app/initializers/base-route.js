import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';

export function initialize(application) {
  Route.reopen({
    feedback: service('feedback'),
    init: function() {
      this._super(...arguments);
      this.set('feedback', {
        execute: this.get('feedback').execute.bind(this),
      });
    },
    // Don't record characters in browser history
    // for the "search" query item (filter)
    // queryParams: {
    //   filter: {
    //     replace: true
    //   }
    // },
    // this is only KV not all Routes
    rootKey: '-',
    actions: {
      // Used to link to keys that are not objects,
      // like parents and grandParents
      // TODO: This is a view thing, should possibly be a helper
      linkToKey: function(key) {
        if (key.slice(-1) === '/' || key === this.rootKey) {
          this.transitionTo('dc.kv.show', key);
        } else {
          this.transitionTo('dc.kv.edit', key);
        }
      },
    },
  });
}

export default {
  initialize,
};
