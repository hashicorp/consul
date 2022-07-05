import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/service';
import { get } from '@ember/object';

export default class ServiceSerializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  respondForQuery(respond, query) {
    return super.respondForQuery(
      cb =>
        respond((headers, body) => {
          // Services and proxies all come together in the same list. Here we
          // map the proxies to their related services on a Service.Proxy
          // property for easy access later on
          const services = {};
          body
            .filter(function(item) {
              return item.Kind !== 'connect-proxy';
            })
            .forEach(item => {
              services[item.Name] = item;
            });
          body
            .filter(function(item) {
              return item.Kind === 'connect-proxy';
            })
            .forEach(item => {
              // Iterating to cover the usecase of a proxy being used by more
              // than one service
              if (item.ProxyFor) {
                item.ProxyFor.forEach(service => {
                  if (typeof services[service] !== 'undefined') {
                    services[service].Proxy = item;
                  }
                });
              }
            });

          return cb(headers, body);
        }),
      query
    );
  }

  respondForQueryRecord(respond, query) {
    // Name is added here from the query, which is used to make the uid
    // Datacenter gets added in the ApplicationSerializer
    return super.respondForQueryRecord(
      cb =>
        respond((headers, body) => {
          return cb(headers, {
            Name: query.id,
            Namespace: get(body, 'firstObject.Service.Namespace'),
            Nodes: body,
          });
        }),
      query
    );
  }
}
