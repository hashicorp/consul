import Adapter from 'ember-data/adapters/rest';
import { inject as service } from '@ember/service';
import { assign } from '@ember/polyfills';
export default Adapter.extend({
  namespace: 'v1',
  repo: service('settings'),
  headersForRequest: function(params) {
    return assign({}, this.get('repo').findHeaders(), this._super(...arguments));
  },
});
