import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/service-instance';

export default class ServiceInstanceSerializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  respondForQuery(respond, query) {
    return super.respondForQuery(function(cb) {
      return respond(function(headers, body) {
        if (body.length === 0) {
          const e = new Error();
          e.errors = [
            {
              status: '404',
              title: 'Not found',
            },
          ];
          throw e;
        }
        return cb(headers, body);
      });
    }, query);
  }

  respondForQueryRecord(respond, query) {
    return super.respondForQueryRecord(function(cb) {
      return respond(function(headers, body) {
        body = body.find(function(item) {
          return item.Node.Node === query.node && item.Service.ID === query.serviceId;
        });
        if (typeof body === 'undefined') {
          const e = new Error();
          e.errors = [
            {
              status: '404',
              title: 'Not found',
            },
          ];
          throw e;
        }
        body.Namespace = body.Service.Namespace;
        return cb(headers, body);
      });
    }, query);
  }
}
