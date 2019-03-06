import Serializer from './application';
import { PRIMARY_KEY } from 'consul-ui/models/service';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  respondForQueryRecord: function(respond, query) {
    return this._super(cb => respond((headers, body) => cb(headers, { Nodes: body })), query);
  },
});
