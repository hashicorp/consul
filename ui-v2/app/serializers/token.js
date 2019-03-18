import Serializer from './application';
import { get } from '@ember/object';
import { PRIMARY_KEY, SLUG_KEY, ATTRS } from 'consul-ui/models/token';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  slugKey: SLUG_KEY,
  // attrs: ATTRS,
  respondForQueryRecord: function(respond, query) {
    return this._super(
      cb =>
        respond((headers, body) => {
          // Sometimes we get `Policies: null`, make null equal an empty array
          if (typeof body.Policies === 'undefined' || body.Policies === null) {
            body.Policies = [];
          }
          // Convert an old style update response to a new style
          if (typeof body['ID'] !== 'undefined') {
            const item = get(this, 'store')
              .peekAll('token')
              .findBy('SecretID', body['ID']);
            if (item) {
              body['SecretID'] = body['ID'];
              body['AccessorID'] = get(item, 'AccessorID');
            }
          }
          return cb(headers, body);
        }),
      query
    );
  },
});
