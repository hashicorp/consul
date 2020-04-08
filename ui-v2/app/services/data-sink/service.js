import Service, { inject as service } from '@ember/service';

const parts = function(uri) {
  if (uri.indexOf('://') === -1) {
    uri = `data://${uri}`;
  }
  const url = new URL(uri);
  let pathname = url.pathname;
  if (pathname.startsWith('//')) {
    pathname = pathname.substr(2);
  }
  const providerName = url.protocol.substr(0, url.protocol.length - 1);
  return [providerName, pathname];
};
export default Service.extend({
  data: service('data-sink/protocols/http'),
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
