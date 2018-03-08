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
