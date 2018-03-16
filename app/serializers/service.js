import Serializer from './application';
export default Serializer.extend({
  primaryKey: 'Id',
  // normalizeQueryResponse: function(store, primaryModelClass, payload, id, requestType) {
  //   return this._super(
  //     store,
  //     primaryModelClass,
  //     {
  //       [primaryModelClass.modelName]: payload,
  //     },
  //     id,
  //     requestType
  //   );
  // },
  // normalizeQueryRecordResponse: function(store, primaryModelClass, payload, id, requestType) {
  //   return this._super(
  //     store,
  //     primaryModelClass,
  //     {
  //       [primaryModelClass.modelName]: payload,
  //     },
  //     id,
  //     requestType
  //   );
  // },
});
