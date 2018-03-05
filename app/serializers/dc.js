import pojo from 'consul-ui/utils/pojo';

import Serializer from './application';
export default Serializer.extend({
  primaryKey: 'Name',
  normalizeFindAllResponse: function(store, primaryModelClass, payload, id, requestType) {
    return this._super(
      store,
      primaryModelClass,
      pojo(primaryModelClass.modelName)(
        payload.map(item => {
          return {
            [this.get('primaryKey')]: item,
          };
        })
      ),
      id,
      requestType
    );
  },
});
