import pojo from 'consul-ui/utils/pojo';

import Serializer from './application';
export default Serializer.extend({
  primaryKey: 'ID',
  normalizeFindRecordResponse: function(store, primaryModelClass, payload, id, requestType) {
    return this._super(
      store,
      primaryModelClass,
      pojo(primaryModelClass.modelName)(payload),
      id,
      requestType
    );
  },
  normalizeFindAllResponse: function(store, primaryModelClass, payload, id, requestType) {
    return this._super(
      store,
      primaryModelClass,
      pojo(primaryModelClass.modelName)(payload),
      id,
      requestType
    );
  },
});
