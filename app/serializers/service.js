import pojo from 'consul-ui/utils/pojo';
import { assign } from '@ember/polyfills';

import Serializer from './application';
export default Serializer.extend({
  primaryKey: 'Id',
  normalizeQueryResponse: function(store, primaryModelClass, payload, id, requestType) {
    return this.normalizeFindAllResponse(...arguments);
  },
  normalizeFindAllResponse: function(store, primaryModelClass, payload, id, requestType) {
    return this._super(
      store,
      primaryModelClass,
      pojo(primaryModelClass.modelName)(
        payload.map(function(item) {
          item.Id = item.Name;
          return item;
        })
      ),
      id,
      requestType
    );
  },
  normalizeFindRecordResponse: function(store, primaryModelClass, payload, id, requestType) {
    return this._super(
      store,
      primaryModelClass,
      pojo(primaryModelClass.modelName)({
        Id: id,
        Nodes: payload,
      }),
      id,
      requestType
    );
  },
});
