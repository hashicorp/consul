import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/session';

export default class SessionSerializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  respondForQueryRecord(respond, query) {
    return super.respondForQueryRecord(
      cb => respond((headers, body) => cb(headers, body[0])),
      query
    );
  }
}
