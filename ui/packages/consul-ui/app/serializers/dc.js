import Serializer from './application';
import { inject as service } from '@ember/service';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/dc';

import {
  HEADERS_SYMBOL,
  HEADERS_DEFAULT_ACL_POLICY as DEFAULT_ACL_POLICY,
} from 'consul-ui/utils/http/consul';
export default class DcSerializer extends Serializer {
  @service('env') env;

  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  // datacenters come in as  an array of plain strings. Convert to objects
  // instead and collect all the other datacenter info from other places and
  // add it to each datacenter object
  respondForQuery(respond, query) {
    return super.respondForQuery(
      cb => respond((headers, body) => {
        body = body.map(item => ({
          Datacenter: '',
          [this.slugKey]: item,
        }));
        body = cb(headers, body);
        headers = body[HEADERS_SYMBOL];

        const Local = this.env.var('CONSUL_DATACENTER_LOCAL');
        const Primary = this.env.var('CONSUL_DATACENTER_PRIMARY');
        const DefaultACLPolicy = headers[DEFAULT_ACL_POLICY.toLowerCase()];

        return body.map(item => ({
          ...item,
          Local: item.Name === Local,
          Primary: item.Name === Primary,
          DefaultACLPolicy: DefaultACLPolicy,
        }));
      }),
      query
    );
  }
}
