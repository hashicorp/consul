import Serializer from './application';

export default Serializer.extend({
  primaryKey: 'ID',
  normalizePayload: function(payload, id, requestType) {
    switch (requestType) {
      case 'queryRecord':
        return payload[0];
    }
    return this._super(...arguments);
  },
});
