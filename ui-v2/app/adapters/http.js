import Adapter from 'ember-data/adapter';
import AdapterError from '@ember-data/adapter/error';
import {
  AbortError,
  TimeoutError,
  ServerError,
  UnauthorizedError,
  ForbiddenError,
  NotFoundError,
  ConflictError,
  InvalidError,
} from 'ember-data/adapters/errors';
// TODO: This is a little skeleton cb function
// is to be replaced soon with something slightly more involved
const responder = function(response) {
  return response;
};
const read = function(adapter, serializer, client, type, query) {
  return client
    .request(function(request) {
      return adapter[`requestFor${type}`](request, query);
    })
    .catch(function(e) {
      return adapter.error(e);
    })
    .then(function(response) {
      return serializer[`respondFor${type}`](responder(response), query);
    });
  // TODO: Potentially add specific serializer errors here
  // .catch(function(e) {
  //   return Promise.reject(e);
  // });
};
const write = function(adapter, serializer, client, type, snapshot) {
  const unserialized = snapshot.attributes();
  const serialized = serializer.serialize(snapshot, {});
  return client
    .request(function(request) {
      return adapter[`requestFor${type}`](request, serialized, unserialized);
    })
    .catch(function(e) {
      return adapter.error(e);
    })
    .then(function(response) {
      return serializer[`respondFor${type}`](responder(response), serialized, unserialized);
    });
  // TODO: Potentially add specific serializer errors here
  // .catch(function(e) {
  //   return Promise.reject(e);
  // });
};
export default Adapter.extend({
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
          error.errors[0].status = '0';
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
    return read(this, store.serializerFor(type.modelName), this.client, 'Query', query);
  },
  queryRecord: function(store, type, query) {
    return read(this, store.serializerFor(type.modelName), this.client, 'QueryRecord', query);
  },
  findAll: function(store, type) {
    return read(this, store.serializerFor(type.modelName), this.client, 'FindAll');
  },
  createRecord: function(store, type, snapshot) {
    return write(this, store.serializerFor(type.modelName), this.client, 'CreateRecord', snapshot);
  },
  updateRecord: function(store, type, snapshot) {
    return write(this, store.serializerFor(type.modelName), this.client, 'UpdateRecord', snapshot);
  },
  deleteRecord: function(store, type, snapshot) {
    return write(this, store.serializerFor(type.modelName), this.client, 'DeleteRecord', snapshot);
  },
});
