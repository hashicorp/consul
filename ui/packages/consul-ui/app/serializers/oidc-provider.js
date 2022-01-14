import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/oidc-provider';

export default class OidcSerializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  respondForAuthorize(respond, serialized, data) {
    // we avoid the parent serializer here as it tries to create a fingerprint
    // for an 'action' request but we still need to pass the headers through
    return respond((headers, body) => {
      return this.attachHeaders(headers, body, data);
    });
  }

  respondForQueryRecord(respond, query) {
    // add the name and nspace here so we can merge this
    // TODO: Look to see if we always want the merging functionality
    return super.respondForQueryRecord(
      cb =>
        respond((headers, body) =>
          cb(headers, {
            Name: query.id,
            Namespace: query.ns,
            Partition: query.partition,
            ...body,
          })
        ),
      query
    );
  }
}
