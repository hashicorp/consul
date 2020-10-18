import Adapter from './http';
import { inject as service } from '@ember/service';

export const DATACENTER_QUERY_PARAM = 'dc';
export const NSPACE_QUERY_PARAM = 'ns';
export default Adapter.extend({
  client: service('client/http'),
  env: service('env'),
  formatNspace: function(nspace) {
    if (this.env.env('CONSUL_NSPACES_ENABLED')) {
      return nspace !== '' ? { [NSPACE_QUERY_PARAM]: nspace } : undefined;
    }
  },
  formatDatacenter: function(dc) {
    return {
      [DATACENTER_QUERY_PARAM]: dc,
    };
  },
});
