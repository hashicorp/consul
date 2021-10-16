import { inject as service } from '@ember/service';
import Serializer from './application';

export default class DcSerializer extends Serializer {
  @service('env') env;

  primaryKey = 'Name';

  respondForQuery(respond, query) {
    return respond(function(headers, body) {
      return {
        body,
        headers,
      };
    });
  }

  normalizePayload(payload, id, requestType) {
    switch (requestType) {
      case 'query':
        return payload.body.map(item => {
          return {
            Local: this.env.var('CONSUL_DATACENTER_LOCAL') === item,
            [this.primaryKey]: item,
            DefaultACLPolicy: payload.headers['x-consul-default-acl-policy'],
          };
        });
    }
    return payload;
  }
}
