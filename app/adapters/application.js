import Adapter from 'ember-data/adapters/rest';
import { inject as service } from '@ember/service';
import { assign } from '@ember/polyfills';
const createURL = function(encoded, raw, encode = encodeURIComponent) {
  return encoded.concat(raw.map(encode)).join('/');
};
export default Adapter.extend({
  namespace: 'v1',
  repo: service('settings'),
  headersForRequest: function(params) {
    return assign({}, this.get('repo').findHeaders(), this._super(...arguments));
  },
  appendURL: function(path, parts = []) {
    return createURL([this.buildURL(), path], parts);
  },
});
