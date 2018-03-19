import Serializer from './application';
export default Serializer.extend({
  primaryKey: 'Key',
  normalizePayload: function(payload, id, requestType) {
    switch (requestType) {
      case 'query':
        return payload.map(item => {
          return {
            Key: item,
          };
        });
      case 'queryRecord':
        return payload[0];
    }
    return this._super(...arguments);
  },
});
