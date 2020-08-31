import Service, { inject as service } from '@ember/service';

const parts = function(uri) {
  if (uri.indexOf('://') === -1) {
    uri = `consul://${uri}`;
  }
  return uri.split('://');
};
export default Service.extend({
  consul: service('data-sink/protocols/http'),
  settings: service('data-sink/protocols/local-storage'),

  prepare: function(uri, data, assign) {
    const [providerName, pathname] = parts(uri);
    const provider = this[providerName];
    return provider.prepare(pathname, data, assign);
  },
  persist: function(uri, data) {
    const [providerName, pathname] = parts(uri);
    const provider = this[providerName];
    return provider.persist(pathname, data);
  },
  remove: function(uri, data) {
    const [providerName, pathname] = parts(uri);
    const provider = this[providerName];
    return provider.remove(pathname, data);
  },
});
