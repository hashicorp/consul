import ApplicationSerializer from './application';
import { typeOf } from '@ember/utils';

const mapKeys = function(payload) {
  return payload.map(function(item, i, arr) {
    return {
      Key: item,
    };
  });
};
const mapValues = function(payload) {
  return payload;
  // const store = this.get('store');
  // return payload.map(
  //   (item, i, arr) => {
  //     const current = store.peekRecord('kv', item.Key);
  //     if(current != null) {
  //       store.unloadRecord(current);
  //     }
  //     return item;
  //   }
  // );
};
export default ApplicationSerializer.extend({
  primaryKey: 'Key',
  normalizeQueryResponse: function(store, primaryModelClass, payload, id, requestType) {
    let map = mapKeys;
    if (payload[0] && typeOf(payload[0]) !== 'string') {
      map = mapValues;
    }
    return this._super(
      store,
      primaryModelClass,
      {
        [primaryModelClass.modelName]: map.bind(this)(payload),
      },
      id,
      requestType
    );
  },
});
