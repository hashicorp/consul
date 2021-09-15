import Serializer from './application';
import { get } from '@ember/object';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/nspace';

export default class NspaceSerializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  respondForQuery(respond, query, data, modelClass) {
    return super.respondForQuery(
      cb =>
        respond((headers, body) =>
          cb(
            headers,
            body.map(function(item) {
              item.Namespace = item.Name;
              if (get(item, 'ACLs.PolicyDefaults')) {
                item.ACLs.PolicyDefaults = item.ACLs.PolicyDefaults.map(function(item) {
                  item.template = '';
                  return item;
                });
              }
              // Both of these might come though unset so we make sure we at least
              // have an empty array here so we can add children to them if we
              // need to whilst saving nspaces
              ['PolicyDefaults', 'RoleDefaults'].forEach(function(prop) {
                if (typeof item.ACLs === 'undefined') {
                  item.ACLs = [];
                }
                if (typeof item.ACLs[prop] === 'undefined') {
                  item.ACLs[prop] = [];
                }
              });
              return item;
            })
          )
        ),
      query
    );
  }
}
