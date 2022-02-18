import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/partition';

export default class PartitionSerializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  respondForQuery(respond, query, data, modelClass) {
    return super.respondForQuery(
      cb =>
        respond((headers, body) => {
          return cb(
            headers,
            body.map(item => {
              item.Partition = '*';
              item.Namespace = '*';
              return item;
            })
          );
        }),
      query
    );
  }
}
