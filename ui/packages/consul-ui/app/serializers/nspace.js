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

  respondForQueryRecord(respond, serialized, data) {
    // We don't attachHeaders here yet, mainly because we don't use blocking
    // queries on form views yet, and by the time we do Serializers should
    // have been refactored to not use attachHeaders
    return respond((headers, body) => {
      return body;
    });
  }

  respondForCreateRecord(respond, serialized, data) {
    return respond((headers, body) => {
      // The data properties sent to be saved in the backend or the same ones
      // that we receive back if its successfull therefore we can just ignore
      // the result and avoid ember-data syncing problems
      return {};
    });
  }

  respondForUpdateRecord(respond, serialized, data) {
    return respond((headers, body) => {
      return body;
    });
  }

  respondForDeleteRecord(respond, serialized, data) {
    return respond((headers, body) => {
      // Deletes only need the primaryKey/uid returning
      return body;
    });
  }
}
