import Serializer from './application';
import { PRIMARY_KEY } from 'consul-ui/models/kv';
import removeNull from 'consul-ui/utils/remove-null';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  respondForQueryRecord: function(respond, query) {
    return this._super(cb => respond((headers, body) => cb(headers, removeNull(body[0]))), query);
  },
});
