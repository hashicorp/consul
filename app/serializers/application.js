import Serializer from 'ember-data/serializers/rest';

export default Serializer.extend({
  normalizeResponse: function(store, primaryModelClass, payload, id, requestType) {
    return this._super(
      store,
      primaryModelClass,
      {
        [primaryModelClass.modelName]: this.normalizePayload(payload, id, requestType),
      },
      id,
      requestType
    );
  },
  normalizePayload: function(payload, id, requestType) {
    return payload;
  },
});
