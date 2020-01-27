import Serializer from './application';
import { inject as service } from '@ember/service';

import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/kv';
import { NSPACE_KEY } from 'consul-ui/models/nspace';
import { NSPACE_QUERY_PARAM as API_NSPACE_KEY } from 'consul-ui/adapters/application';
import removeNull from 'consul-ui/utils/remove-null';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  slugKey: SLUG_KEY,
  decoder: service('atob'),
  // TODO: Would undefined be better instead of null?
  serialize: function(snapshot, options) {
    const value = snapshot.attr('Value');
    return typeof value === 'string' ? this.decoder.execute(value) : null;
  },
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
                [NSPACE_KEY]: query[API_NSPACE_KEY],
              };
            })
          );
        }),
      query
    );
  },
});
