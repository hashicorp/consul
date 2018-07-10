export default function(type) {
  let requests = null;
  switch (type) {
    case 'dc':
      requests = ['/v1/catalog/datacenters'];
      break;
    case 'service':
      requests = ['/v1/internal/ui/services', '/v1/health/service/'];
      break;
    case 'node':
      requests = ['/v1/internal/ui/nodes'];
      break;
    case 'kv':
      requests = ['/v1/kv/'];
      break;
    case 'acl':
      requests = ['/v1/acl/list'];
      break;
    case 'session':
      requests = ['/v1/session/node/'];
      break;
  }
  // TODO: An instance of URL should come in here (instead of 2 args)
  return function(url, method) {
    if (requests === null) {
      return false;
    }
    return requests.some(function(item) {
      return method.toUpperCase() === 'GET' && url.indexOf(item) === 0;
    });
  };
}
