export default function(type, count, obj) {
  var key = '';
  obj['CONSUL_ACLS_ENABLE'] = 1;
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
      obj['CONSUL_ACLS_ENABLE'] = 1;
      break;
    case 'session':
      key = 'CONSUL_SESSION_COUNT';
      break;
    case 'intention':
      key = 'CONSUL_INTENTION_COUNT';
      break;
    case 'policy':
      key = 'CONSUL_POLICY_COUNT';
      obj['CONSUL_ACLS_ENABLE'] = 1;
      break;
    case 'token':
      key = 'CONSUL_TOKEN_COUNT';
      obj['CONSUL_ACLS_ENABLE'] = 1;
      break;
  }
  if (key) {
    obj[key] = count;
  }
  return obj;
}
