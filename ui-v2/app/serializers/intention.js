import Serializer from './application';
import { inject as service } from '@ember/service';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/intention';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  slugKey: SLUG_KEY,
  encoder: service('encoder'),
  init: function() {
    this._super();
    this.uri = this.encoder.uriTag();
  },
  ensureID: function(item) {
    if (typeof item.ID !== 'string') {
      item.ID = this
        .uri`${item.SourceNS}:${item.SourceName}:${item.DestinationNS}:${item.DestinationName}`;
      item.Legacy = false;
    } else {
      item.Legacy = true;
    }
    return item;
  },
  respondForQuery: function(respond, query) {
    return this._super(
      cb =>
        respond((headers, body) => {
          return cb(
            headers,
            body.map(item => this.ensureID(item))
          );
        }),
      query
    );
  },
  respondForQueryRecord: function(respond, query) {
    return this._super(
      cb =>
        respond((headers, body) => {
          body = this.ensureID(body);
          return cb(headers, body);
        }),
      query
    );
  },
});
