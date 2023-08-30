import { inject as service } from '@ember/service';
import Adapter from '@ember-data/adapter';
import AdapterError, {
  AbortError,
  TimeoutError,
  ServerError,
  UnauthorizedError,
  ForbiddenError,
  NotFoundError,
  ConflictError,
  InvalidError,
} from '@ember-data/adapter/error';

// TODO These are now exactly the same, apart from the fact that one uses
// `serialized, unserialized` and the other just `query`
// they could actually be one function now, but would be nice to think about
// the naming of things (serialized vs query etc)
const read = function (adapter, modelName, type, query = {}) {
  return adapter.rpc(
    function (adapter, ...rest) {
      return adapter[`requestFor${type}`](...rest);
    },
    function (serializer, ...rest) {
      return serializer[`respondFor${type}`](...rest);
    },
    query,
    modelName
  );
};
const write = function (adapter, modelName, type, snapshot) {
  return adapter.rpc(
    function (adapter, ...rest) {
      return adapter[`requestFor${type}`](...rest);
    },
    function (serializer, ...rest) {
      return serializer[`respondFor${type}`](...rest);
    },
    snapshot,
    modelName
  );
};
export default class HttpAdapter extends Adapter {
  @service('client/http') client;

  rpc(req, resp, obj, modelName) {
    const client = this.client;
    const store = this.store;
    const adapter = this;

    let unserialized, serialized;
    const serializer = store.serializerFor(modelName);
    const modelClass = store.modelFor(modelName);
    // workable way to decide whether this is a snapshot
    // essentially 'is attributable'.
    // Snapshot is private so we can't do instanceof here
    // and using obj.constructor.name gets changed/minified
    // during compilation so you can't rely on it
    // checking for `attributes` being a function is more
    // reliable as that is the thing we need to call
    if (typeof obj.attributes === 'function') {
      unserialized = obj.attributes();
      serialized = serializer.serialize(obj, {});
    } else {
      unserialized = obj;
      serialized = unserialized;
    }

    return client
      .request(function (request) {
        return req(adapter, request, serialized, unserialized, modelClass);
      })
      .catch(function (e) {
        return adapter.error(e);
      })
      .then(function (respond) {
        // TODO: When HTTPAdapter:responder changes, this will also need to change
        return resp(serializer, respond, serialized, unserialized, modelClass);
      });
    // TODO: Potentially add specific serializer errors here
    // .catch(function(e) {
    //   return Promise.reject(e);
    // });
  }

  error(err) {
    if (err instanceof TypeError) {
      throw err;
    }
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
    // TODO: This comes originates from ember-data
    // This can be confusing if you need to use this with Promise.reject
    // Consider changing this to return the error and then
    // throw from the call site instead
    throw error;
  }

  query(store, type, query) {
    return read(this, type.modelName, 'Query', query);
  }

  queryRecord(store, type, query) {
    return read(this, type.modelName, 'QueryRecord', query);
  }

  findAll(store, type) {
    return read(this, type.modelName, 'FindAll');
  }

  createRecord(store, type, snapshot) {
    return write(this, type.modelName, 'CreateRecord', snapshot);
  }

  updateRecord(store, type, snapshot) {
    return write(this, type.modelName, 'UpdateRecord', snapshot);
  }

  deleteRecord(store, type, snapshot) {
    return write(this, type.modelName, 'DeleteRecord', snapshot);
  }
}
