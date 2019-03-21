import Adapter from 'ember-data/adapter';
import { get } from '@ember/object';

export default Adapter.extend({
  snapshotToJSON: function(snapshot, type, options) {
    return snapshot.attributes();
  },
  query: function(store, type, query) {
    const serializer = store.serializerFor(type.modelName);
    return get(this, 'client')
      .request(request => this.requestForQuery(request, query))
      .then(respond => serializer.respondForQuery(respond, query, type));
  },
  queryRecord: function(store, type, query) {
    const serializer = store.serializerFor(type.modelName);
    return get(this, 'client')
      .request(request => this.requestForQueryRecord(request, query))
      .then(respond => serializer.respondForQueryRecord(respond, query));
  },
  findAll: function(store, type) {
    const serializer = store.serializerFor(type.modelName);
    return get(this, 'client')
      .request(request => this.requestForFindAll(request))
      .then(respond => serializer.respondForFindAll(respond));
  },
  createRecord: function(store, type, snapshot) {
    const serializer = store.serializerFor(type.modelName);
    const unserialized = this.snapshotToJSON(snapshot, type);
    const serialized = serializer.serialize(snapshot, {});
    return get(this, 'client')
      .request(request => this.requestForCreateRecord(request, unserialized), serialized)
      .then(respond => serializer.respondForCreateRecord(respond, unserialized, type));
  },
  updateRecord: function(store, type, snapshot) {
    const serializer = store.serializerFor(type.modelName);
    const unserialized = this.snapshotToJSON(snapshot, type);
    const serialized = serializer.serialize(snapshot, {});
    return get(this, 'client')
      .request(request => this.requestForUpdateRecord(request, unserialized), serialized)
      .then(respond => serializer.respondForUpdateRecord(respond, unserialized, type));
  },
  deleteRecord: function(store, type, snapshot) {
    const serializer = store.serializerFor(type.modelName);
    const unserialized = this.snapshotToJSON(snapshot, type);
    const serialized = serializer.serialize(snapshot, {});
    return get(this, 'client')
      .request(request => this.requestForDeleteRecord(request, unserialized), serialized)
      .then(respond => serializer.respondForDeleteRecord(respond, unserialized, type));
  },
});
