export default function(type, count, obj) {
  var key = '';
  switch (type) {
    case 'dc':
      key = 'CONSUL_DATACENTER_COUNT';
      break;
    case 'service':
      key = 'CONSUL_SERVICE_COUNT';
      break;
    case 'node':
      key = 'CONSUL_NODE_COUNT';
      break;
    case 'kv':
      key = 'CONSUL_KV_COUNT';
      break;
    case 'acl':
      key = 'CONSUL_ACL_COUNT';
      obj['CONSUL_ENABLE_ACLS'] = 1;
      break;
    case 'session':
      key = 'CONSUL_SESSION_COUNT';
      break;
    case 'intention':
      key = 'CONSUL_INTENTION_COUNT';
      break;
  }
  if (key) {
    obj[key] = count;
  }
  return obj;
}
