export default function(type) {
  let url = null;
  switch (type) {
    case 'dc':
      url = ['/v1/catalog/datacenters'];
      break;
    case 'service':
      url = ['/v1/internal/ui/services', '/v1/health/service/'];
      break;
    case 'node':
      url = ['/v1/internal/ui/nodes'];
      break;
    case 'kv':
      url = '/v1/kv/';
      break;
    case 'acl':
      url = ['/v1/acl/list'];
      break;
    case 'session':
      url = ['/v1/session/node/'];
      break;
  }
  return function(actual) {
    if (url === null) {
      return false;
    }
    if (typeof url === 'string') {
      return url === actual;
    }
    return url.some(function(item) {
      return actual.indexOf(item) === 0;
    });
  };
}
