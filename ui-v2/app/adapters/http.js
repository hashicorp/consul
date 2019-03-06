import Adapter from 'ember-data/adapter';
import { get } from '@ember/object';

export default Adapter.extend({
  snapshotToJSON: function(store, snapshot, type, opts) {
    const serialized = {};
    const serializer = store.serializerFor(type.modelName);
    // TODO: return?
    serializer.serializeIntoHash(serialized, type, snapshot, opts);

    return serialized[type.modelName];
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
    const data = this.snapshotToJSON(store, snapshot, type, { includeId: true });
    return get(this, 'client')
      .request(request => this.requestForCreateRecord(request, data))
      .then(respond => serializer.respondForCreateRecord(respond, data, type));
  },
  updateRecord: function(store, type, snapshot) {
    const serializer = store.serializerFor(type.modelName);
    const data = this.snapshotToJSON(store, snapshot, type);
    return get(this, 'client')
      .request(request => this.requestForUpdateRecord(request, data))
      .then(respond => serializer.respondForUpdateRecord(respond, data, type));
  },
  deleteRecord: function(store, type, snapshot) {
    const serializer = store.serializerFor(type.modelName);
    const data = this.snapshotToJSON(store, snapshot, type);
    return get(this, 'client')
      .request(request => this.requestForDeleteRecord(request, data))
      .then(respond => serializer.respondForDeleteRecord(respond, data, type));
  },
});
