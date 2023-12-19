import Service, { inject as service } from '@ember/service';

const parts = function (uri) {
  uri = uri.toString();
  if (uri.indexOf('://') === -1) {
    uri = `consul://${uri}`;
  }
  return uri.split('://');
};
export default class DataSinkService extends Service {
  @service('data-sink/protocols/http') consul;
  @service('data-sink/protocols/local-storage') settings;

  prepare(uri, data, assign) {
    const [providerName, pathname] = parts(uri);
    const provider = this[providerName];
    return provider.prepare(pathname, data, assign);
  }

  persist(uri, data) {
    const [providerName, pathname] = parts(uri);
    const provider = this[providerName];
    return provider.persist(pathname, data);
  }

  remove(uri, data) {
    const [providerName, pathname] = parts(uri);
    const provider = this[providerName];
    return provider.remove(pathname, data);
  }
}
