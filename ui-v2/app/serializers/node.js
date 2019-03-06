import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/node';

// TODO: Looks like ID just isn't used at all
// consider just using .Node for the SLUG_KEY
const fillSlug = function(item) {
  if (item[SLUG_KEY] === '') {
    item[SLUG_KEY] = item['Node'];
  }
  return item;
};

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  respondForQuery: function(respond, query) {
    return this._super(
      cb => respond((headers, body) => cb(headers, { Nodes: body.map(fillSlug) })),
      query
    );
  },
});
