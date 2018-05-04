import Serializer from 'ember-data/serializers/rest';

export default Serializer.extend({
  // this could get confusing if you tried to override
  // say `normalizeQueryResponse`
  // TODO: consider creating a method for each one of the `normalize...Response` family
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
