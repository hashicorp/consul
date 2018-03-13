import ApplicationSerializer from './application';
import { typeOf } from '@ember/utils';
export default ApplicationSerializer.extend({
  primaryKey: 'ID',
  normalizeQueryResponse: function(store, primaryModelClass, payload, id, requestType) {
    return this._super(
      store,
      primaryModelClass,
      {
        [primaryModelClass.modelName]: payload,
      },
      id,
      requestType
    );
  },
  normalizeQueryRecordResponse: function(store, primaryModelClass, payload, id, requestType) {
    return this._super(
      store,
      primaryModelClass,
      {
        [primaryModelClass.modelName]: typeOf(payload) === 'array' ? payload[0] : payload,
      },
      id,
      requestType
    );
  },
  normalizeCreateRecordResponse: function(store, primaryModelClass, payload, id, requestType) {
    return this._super(
      store,
      primaryModelClass,
      {
        [primaryModelClass.modelName]: payload,
      },
      id,
      requestType
    );
  },
});
