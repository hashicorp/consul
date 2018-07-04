import index from 'consul-ui/tests/pages/index';
import dcs from 'consul-ui/tests/pages/dc';
import settings from 'consul-ui/tests/pages/settings';
import services from 'consul-ui/tests/pages/dc/services/index';
import service from 'consul-ui/tests/pages/dc/services/show';
import nodes from 'consul-ui/tests/pages/dc/nodes/index';
import node from 'consul-ui/tests/pages/dc/nodes/show';
import kvs from 'consul-ui/tests/pages/dc/kv/index';
import kv from 'consul-ui/tests/pages/dc/kv/edit';
import acls from 'consul-ui/tests/pages/dc/acls/index';
import acl from 'consul-ui/tests/pages/dc/acls/edit';
import intentions from 'consul-ui/tests/pages/dc/intentions/index';
import intention from 'consul-ui/tests/pages/dc/intentions/edit';

export default {
  index,
  dcs,
  settings,
  services,
  service,
  nodes,
  node,
  kvs,
  kv,
  acls,
  acl,
  intentions,
  intention,
};
