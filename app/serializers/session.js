import ApplicationSerializer from './application';

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
});
