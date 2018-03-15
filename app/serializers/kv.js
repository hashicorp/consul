import Serializer from './application';

export default Serializer.extend({
  primaryKey: 'Key',
  normalizeQueryResponse: function(store, primaryModelClass, payload, id, requestType) {
    return this._super(
      store,
      primaryModelClass,
      {
        [primaryModelClass.modelName]: payload.map(function(item, i, arr) {
          return {
            Key: item,
          };
        }),
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
  normalizeQueryRecordResponse: function(store, primaryModelClass, payload, id, requestType) {
    return this._super(
      store,
      primaryModelClass,
      {
        [primaryModelClass.modelName]: payload[0],
      },
      id,
      requestType
    );
  },
});
