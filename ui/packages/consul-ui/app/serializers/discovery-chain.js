import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/discovery-chain';

export default class DiscoveryChainSerializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  respondForQueryRecord(respond, query) {
    return super.respondForQueryRecord(function(cb) {
      return respond(function(headers, body) {
        return cb(headers, {
          ...body,
          [SLUG_KEY]: body.Chain[SLUG_KEY],
        });
      });
    }, query);
  }
}
