import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/kv';
import removeNull from 'consul-ui/utils/remove-null';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  slugKey: SLUG_KEY,
  respondForQueryRecord: function(respond, query) {
    return this._super(cb => respond((headers, body) => cb(headers, removeNull(body[0]))), query);
  },
  respondForQuery: function(respond, query) {
    return this._super(
      cb =>
        respond((headers, body) => {
          return cb(
            headers,
            body.map(item => {
              return {
                [this.slugKey]: item,
              };
            })
          );
        }),
      query
    );
  },
});
