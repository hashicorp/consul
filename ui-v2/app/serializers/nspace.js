import Serializer from './application';
import { get } from '@ember/object';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/nspace';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  slugKey: SLUG_KEY,
  respondForQuery: function(respond, serialized, data) {
    return respond((headers, body) => {
      return this.attachHeaders(
        headers,
        body.map(function(item) {
          if (get(item, 'ACLs.PolicyDefaults')) {
            item.ACLs.PolicyDefaults = item.ACLs.PolicyDefaults.map(function(item) {
              item.template = '';
              return item;
            });
          }
          // Both of these might come though unset so we make sure
          // we at least have an empty array here so we can add
          // children to them if we need to whilst saving nspaces
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
      );
    });
  },
  respondForQueryRecord: function(respond, serialized, data) {
    // We don't attachHeaders here yet, mainly because we don't use
    // blocking queries on form views yet, and by the time we do
    // Serializers should have been refactored to not use attachHeaders
    return respond((headers, body) => {
      return body;
    });
  },
  respondForCreateRecord: function(respond, serialized, data) {
    return respond((headers, body) => {
      // potentially this could be default functionality
      // we are basically making the assumption here that if a create
      // comes back with a record then we should prefer that over the local
      // one seeing as it has come from the backend, could we potenitally lose
      // data that is stored in the frontend here?
      // if we were to do this as a default thing we'd have to use uid here
      // which we don't know until its gone through the serializer after this
      const item = this.store.peekRecord('nspace', body.Name);
      if (item) {
        this.store.unloadRecord(item);
      }
      return body;
    });
  },
  respondForUpdateRecord: function(respond, serialized, data) {
    return respond((headers, body) => {
      return body;
    });
  },
  respondForDeleteRecord: function(respond, serialized, data) {
    return respond((headers, body) => {
      // Deletes only need the primaryKey/uid returning
      return body;
    });
  },
});
