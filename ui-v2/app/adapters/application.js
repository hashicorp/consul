import Adapter from './http';
import { inject as service } from '@ember/service';
// TODO: This should be changed to use env
import config from 'consul-ui/config/environment';

export const DATACENTER_QUERY_PARAM = 'dc';
export const NSPACE_QUERY_PARAM = 'ns';
export default Adapter.extend({
  repo: service('settings'),
  client: service('client/http'),
  formatNspace: function(nspace) {
    if (config.CONSUL_NSPACES_ENABLED) {
      return nspace !== '' ? { [NSPACE_QUERY_PARAM]: nspace } : undefined;
    }
  },
  formatDatacenter: function(dc) {
    return {
      [DATACENTER_QUERY_PARAM]: dc,
    };
  },
  // TODO: Deprecated, remove `request` usage from everywhere and replace with
  // `HTTPAdapter.rpc`
  request: function(req, resp, obj, modelName) {
    return this.rpc(...arguments);
  },
});
