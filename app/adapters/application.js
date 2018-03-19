import Adapter from 'ember-data/adapters/rest';
import { inject as service } from '@ember/service';
import { assign } from '@ember/polyfills';
import createURL from 'consul-ui/utils/createURL';
export default Adapter.extend({
  namespace: 'v1',
  repo: service('settings'),
  headersForRequest: function(params) {
    return assign({}, this.get('repo').findHeaders(), this._super(...arguments));
  },

  // appendURL in turn calls createURL
  // createURL ensures that all `parts` are URL encoded
  // and all `query` values are URL encoded

  // `this.buildURL()` with no arguments will give us `${host}/${namespace}`
  // `path` is the user configurable 'urlsafe' string to append on `buildURL`
  // `parts` is an array of possibly non 'urlsafe parts' to be encoded and
  // appended onto the url
  // `query` will populate the query string. Again the values of which will be
  // url encoded

  appendURL: function(path, parts = [], query = {}) {
    return createURL([this.buildURL(), path], parts, query);
  },
});
