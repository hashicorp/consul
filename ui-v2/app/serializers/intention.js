import Serializer from './application';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';
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
    if (!get(item, 'ID.length')) {
      item.Legacy = false;
    } else {
      item.Legacy = true;
      item.LegacyID = item.ID;
    }
    item.ID = this
      .uri`${item.SourceNS}:${item.SourceName}:${item.DestinationNS}:${item.DestinationName}`;
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
  respondForUpdateRecord: function(respond, serialized, data) {
    return this._super(
      cb =>
        respond((headers, body) => {
          body.LegacyID = body.ID;
          body.ID = serialized.ID;
          return cb(headers, body);
        }),
      serialized,
      data
    );
  },
});
