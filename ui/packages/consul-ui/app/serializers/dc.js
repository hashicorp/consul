import Serializer from './application';

export default class DcSerializer extends Serializer {
  primaryKey = 'Name';

  respondForQuery(respond, query) {
    return respond(function(headers, body) {
      return body;
    });
  }

  normalizePayload(payload, id, requestType) {
    switch (requestType) {
      case 'query':
        return payload.map(item => {
          return {
            [this.primaryKey]: item,
          };
        });
    }
    return payload;
  }
}
