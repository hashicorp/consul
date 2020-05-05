import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/oidc-provider';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
  slugKey: SLUG_KEY,
  respondForAuthorize: function(respond, serialized, data) {
    // we avoid the parent serializer here as it tries to create a
    // fingerprint for an 'action' request
    // but we still need to pass the headers through
    return respond((headers, body) => {
      return this.attachHeaders(headers, body, data);
    });
  },
  respondForQueryRecord: function(respond, query) {
    // add the name and nspace here so we can merge this
    // TODO: Look to see if we always want the merging functionality
    return this._super(
      cb =>
        respond((headers, body) =>
          cb(headers, {
            Name: query.id,
            Namespace: query.ns,
            ...body,
          })
        ),
      query
    );
  },
});
