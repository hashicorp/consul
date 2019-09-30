import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/service';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  slugKey: SLUG_KEY,
  respondForQueryRecord: function(respond, query) {
    // Name is added here from the query, which is used to make the uid
    // Datacenter gets added in the ApplicationSerializer
    return this._super(
      cb => respond((headers, body) => cb(headers, { Name: query.id, Nodes: body })),
      query
    );
  },
});
