import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/gateway';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  slugKey: SLUG_KEY,
  respondForQueryRecord: function(respond, query) {
    return this._super(function(cb) {
      return respond(function(headers, body) {
        return cb(headers, {
          Name: query.id,
          Services: body,
        });
      });
    }, query);
  },
});
