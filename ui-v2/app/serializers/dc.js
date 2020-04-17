import Serializer from './application';

export default Serializer.extend({
  primaryKey: 'Name',
  respondForQuery: function(respond, query) {
    return respond(function(headers, body) {
      return body;
    });
  },
  normalizePayload: function(payload, id, requestType) {
    switch (requestType) {
      case 'query':
      case 'findAll':
        return payload.map(item => {
          return {
            [this.primaryKey]: item,
          };
        });
    }
    return payload;
  },
});
