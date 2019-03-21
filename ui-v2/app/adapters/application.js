import Adapter from './http';
import { inject as service } from '@ember/service';

export const DATACENTER_QUERY_PARAM = 'dc';
export default Adapter.extend({
  repo: service('settings'),
  client: service('client/http'),
});
