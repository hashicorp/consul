import Serializer from './application';
import { inject as service } from '@ember/service';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/kv';

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
      cb =>
        respond((headers, body) => {
          // If item.Session is not set make sure we overwrite any existing one.
          // Using @replace, defaultValue or similar model apporaches does not work
          // as if a property is undefined ember-data just ignores it instead of
          // deleting the value of the existing property.
          if (typeof body[0].Session === 'undefined') {
            body[0].Session = '';
          }
          //
          return cb(headers, body[0]);
        }),
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
              };
            })
          );
        }),
      query
    );
  }
}
