import Serializer from './application';
import { typeOf } from '@ember/utils';
export default Serializer.extend({
  primaryKey: 'ID',
  normalizePayload: function(payload, id, requestType) {
    switch (requestType) {
      case 'deleteRecord':
        return { [this.get('primaryKey')]: id };
      case 'queryRecord':
        return typeOf(payload) === 'array' ? payload[0] : payload;
    }
    return payload;
  },
});
