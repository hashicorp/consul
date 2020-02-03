import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/service';
import { get } from '@ember/object';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  slugKey: SLUG_KEY,
  respondForQueryRecord: function(respond, query) {
    // Name is added here from the query, which is used to make the uid
    // Datacenter gets added in the ApplicationSerializer
    return this._super(
      cb =>
        respond((headers, body) => {
          return cb(headers, {
            Name: query.id,
            Namespace: get(body, 'firstObject.Service.Namespace'),
            Nodes: body,
          });
        }),
      query
    );
  },
});
