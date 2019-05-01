export default function(type) {
  let requests = null;
  switch (type) {
    case 'dc':
      requests = ['/v1/catalog/datacenters'];
      break;
    case 'service':
    case 'instance':
      requests = ['/v1/internal/ui/services', '/v1/health/service/'];
      break;
    case 'proxy':
      requests = ['/v1/catalog/connect'];
      break;
    case 'node':
      requests = ['/v1/internal/ui/nodes', '/v1/internal/ui/node/'];
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
    case 'policy':
      requests = ['/v1/acl/policies', '/v1/acl/policy/'];
      break;
    case 'role':
      requests = ['/v1/acl/roles', '/v1/acl/role/'];
      break;
    case 'token':
      requests = ['/v1/acl/tokens', '/v1/acl/token/'];
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
