export default function(type) {
  let url = null;
  switch (type) {
    case 'dc':
      url = '/v1/catalog/datacenters';
      break;
    case 'service':
      url = '/v1/internal/ui/services';
      break;
    case 'node':
      url = '/v1/internal/ui/nodes';
      // url = '/v1/health/service/_';
      break;
    case 'kv':
      url = '/v1/kv/';
      break;
    case 'acl':
      url = '/v1/acl/list';
      // url = '/v1/acl/info/_';
      break;
  }
  return url;
}
