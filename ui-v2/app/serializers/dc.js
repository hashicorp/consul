import Serializer from './application';
import { get } from '@ember/object';

export default Serializer.extend({
  primaryKey: 'Name',
  normalizePayload: function(payload, id, requestType) {
    switch (requestType) {
      case 'findAll':
        return payload.map(item => {
          return {
            [get(this, 'primaryKey')]: item,
          };
        });
    }
    return payload;
  },
});
