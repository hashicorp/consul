import Serializer from './application';
export default Serializer.extend({
  primaryKey: 'Name',
  normalizePayload: function(payload, id, requestType) {
    switch (requestType) {
      case 'findAll':
        return payload.map(item => {
          return {
            [this.get('primaryKey')]: item,
          };
        });
    }
    return this._super(...arguments);
  },
});
