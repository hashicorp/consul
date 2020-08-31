import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/acl';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  slugKey: SLUG_KEY,
  respondForQueryRecord: function(respond, query) {
    return this._super(cb => respond((headers, body) => cb(headers, body[0])), query);
  },
});
