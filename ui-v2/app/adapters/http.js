import Adapter from 'ember-data/adapter';
import {
  AbortError,
  TimeoutError,
  ServerError,
  UnauthorizedError,
  ForbiddenError,
  NotFoundError,
  ConflictError,
  InvalidError,
  AdapterError,
} from 'ember-data/adapters/errors';
import { get } from '@ember/object';

export default Adapter.extend({
  snapshotToJSON: function(snapshot, type, options) {
    return snapshot.attributes();
  },
  error: function(err) {
    const errors = [
      {
        status: `${err.statusCode}`,
        title: 'The backend responded with an error',
        detail: err.message,
      },
    ];
    let error;
    const detailedMessage = '';
    try {
      switch (err.statusCode) {
        case 0:
          error = new AbortError();
          break;
        case 401:
          error = new UnauthorizedError(errors, detailedMessage);
          break;
        case 403:
          error = new ForbiddenError(errors, detailedMessage);
          break;
        case 404:
          error = new NotFoundError(errors, detailedMessage);
          break;
        case 408:
          error = new TimeoutError();
          break;
        case 409:
          error = new ConflictError(errors, detailedMessage);
          break;
        case 422:
          error = new InvalidError(errors); //payload.errors
          break;
        default:
          if (err.statusCode >= 500) {
            error = new ServerError(errors, detailedMessage);
          } else {
            error = new AdapterError(errors, detailedMessage);
          }
      }
    } catch (e) {
      error = e;
    }
    throw error;
  },
  query: function(store, type, query) {
    const serializer = store.serializerFor(type.modelName);
    return get(this, 'client')
      .request(request => this.requestForQuery(request, query))
      .catch(e => this.error(e))
      .then(respond => serializer.respondForQuery(respond, query, type));
  },
  queryRecord: function(store, type, query) {
    const serializer = store.serializerFor(type.modelName);
    return get(this, 'client')
      .request(request => this.requestForQueryRecord(request, query))
      .catch(e => this.error(e))
      .then(respond => serializer.respondForQueryRecord(respond, query));
  },
  findAll: function(store, type) {
    const serializer = store.serializerFor(type.modelName);
    return get(this, 'client')
      .request(request => this.requestForFindAll(request))
      .catch(e => this.error(e))
      .then(respond => serializer.respondForFindAll(respond));
  },
  createRecord: function(store, type, snapshot) {
    const serializer = store.serializerFor(type.modelName);
    const unserialized = this.snapshotToJSON(snapshot, type);
    const serialized = serializer.serialize(snapshot, {});
    return get(this, 'client')
      .request(request => this.requestForCreateRecord(request, unserialized), serialized)
      .catch(e => this.error(e))
      .then(respond => serializer.respondForCreateRecord(respond, unserialized, type));
  },
  updateRecord: function(store, type, snapshot) {
    const serializer = store.serializerFor(type.modelName);
    const unserialized = this.snapshotToJSON(snapshot, type);
    const serialized = serializer.serialize(snapshot, {});
    return get(this, 'client')
      .request(request => this.requestForUpdateRecord(request, unserialized), serialized)
      .catch(e => this.error(e))
      .then(respond => serializer.respondForUpdateRecord(respond, unserialized, type));
  },
  deleteRecord: function(store, type, snapshot) {
    const serializer = store.serializerFor(type.modelName);
    const unserialized = this.snapshotToJSON(snapshot, type);
    const serialized = serializer.serialize(snapshot, {});
    return get(this, 'client')
      .request(request => this.requestForDeleteRecord(request, unserialized), serialized)
      .catch(e => this.error(e))
      .then(respond => serializer.respondForDeleteRecord(respond, unserialized, type));
  },
});
