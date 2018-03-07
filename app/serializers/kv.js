import ApplicationSerializer from './application';

export default ApplicationSerializer.extend({
  primaryKey: 'Node',
  normalizeQueryResponse: function(store, primaryModelClass, payload, id, requestType) {
    return this.normalizeFindAllResponse(...arguments);
  },
  normalizeFindAllResponse: function(store, primaryModelClass, payload, id, requestType) {
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
  normalizeFindRecordResponse: function(store, primaryModelClass, payload, id, requestType) {
    // this feels strange but prefer non-repetition
    return this.normalizeFindAllResponse(...arguments);
  },
});
