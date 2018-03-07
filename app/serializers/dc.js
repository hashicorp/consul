import Serializer from './application';
export default Serializer.extend({
  primaryKey: 'Name',
  normalizeFindAllResponse: function(store, primaryModelClass, payload, id, requestType) {
    return this._super(
      store,
      primaryModelClass,
      {
        [primaryModelClass.modelName]: payload.map(item => {
          return {
            [this.get('primaryKey')]: item,
          };
        }),
      },
      id,
      requestType
    );
  },
});
