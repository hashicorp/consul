import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/node';

// TODO: Looks like ID just isn't used at all consider just using .Node for
// the SLUG_KEY
const fillSlug = function(item) {
  if (item[SLUG_KEY] === '') {
    item[SLUG_KEY] = item['Node'];
  }
  return item;
};

export default class NodeSerializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  respondForQuery(respond, query) {
    return super.respondForQuery(
      cb => respond((headers, body) => cb(headers, body.map(fillSlug))),
      query
    );
  }

  respondForQueryRecord(respond, query) {
    return super.respondForQueryRecord(
      cb =>
        respond((headers, body) => {
          return cb(headers, fillSlug(body));
        }),
      query
    );
  }

  respondForQueryLeader(respond, query) {
    // don't call super here we don't care about
    // ids/fingerprinting
    return respond((headers, body) => {
      // This response/body is just an ip:port like `"10.0.0.1:8500"`
      // split it and make it look like a `C`onsul.`R`esponse
      // popping off the end for ports should cover us for IPv6 addresses
      // as we should always get a `address:port` or `[a:dd:re:ss]:port` combo
      const temp = body.split(':');
      const port = temp.pop();
      const address = temp.join(':');
      return this.attachHeaders(
        headers,
        {
          Address: address,
          Port: port,
        },
        query
      );
    });
  }
}
