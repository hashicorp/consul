import Adapter from './http';
import { inject as service } from '@ember/service';

export const DATACENTER_QUERY_PARAM = 'dc';
export const NSPACE_QUERY_PARAM = 'ns';

export default class ApplicationAdapter extends Adapter {
  @service('client/http') client;
  @service('env') env;

  formatNspace(nspace) {
    if (this.env.var('CONSUL_NSPACES_ENABLED')) {
      return nspace !== '' ? { [NSPACE_QUERY_PARAM]: nspace } : undefined;
    }
  }

  formatDatacenter(dc) {
    return {
      [DATACENTER_QUERY_PARAM]: dc,
    };
  }
}
