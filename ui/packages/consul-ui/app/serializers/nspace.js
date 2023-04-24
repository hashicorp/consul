import Serializer from './application';
import { get } from '@ember/object';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/nspace';

const normalizeACLs = item => {
  if (get(item, 'ACLs.PolicyDefaults')) {
    item.ACLs.PolicyDefaults = item.ACLs.PolicyDefaults.map(function(item) {
      if (typeof item.template === 'undefined') {
        item.template = '';
      }
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
};

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
              item.Namespace = '*';
              item.Datacenter = query.dc;
              return normalizeACLs(item);
            })
          )
        ),
      query
    );
  }

  respondForQueryRecord(respond, serialized, data) {
    return super.respondForQuery(
      cb =>
        respond((headers, body) => {
          body.Datacenter = serialized.dc;
          body.Namespace = '*';
          return cb(headers, normalizeACLs(body));
        }),
      serialized,
      data
    );
  }

  respondForCreateRecord(respond, serialized, data) {
    return super.respondForCreateRecord(
      cb =>
        respond((headers, body) => {
          body.Datacenter = serialized.dc;
          body.Namespace = '*';
          return cb(headers, normalizeACLs(body));
        }),
      serialized,
      data
    );
  }

  respondForUpdateRecord(respond, serialized, data) {
    return respond((headers, body) => {
      return normalizeACLs(body);
    });
  }
}
