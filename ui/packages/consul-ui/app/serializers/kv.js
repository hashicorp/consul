import Serializer from './application';
import { inject as service } from '@ember/service';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/kv';
import { NSPACE_KEY } from 'consul-ui/models/nspace';
import { NSPACE_QUERY_PARAM as API_NSPACE_KEY } from 'consul-ui/adapters/application';

export default class KvSerializer extends Serializer {
  @service('atob') decoder;

  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  // TODO: Would undefined be better instead of null?
  serialize(snapshot, options) {
    const value = snapshot.attr('Value');
    return typeof value === 'string' ? this.decoder.execute(value) : null;
  }

  respondForQueryRecord(respond, query) {
    return super.respondForQueryRecord(
      cb => respond((headers, body) => cb(headers, body[0])),
      query
    );
  }

  respondForQuery(respond, query) {
    return super.respondForQuery(
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
  }
}
