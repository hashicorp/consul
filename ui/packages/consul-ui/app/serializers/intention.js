import Serializer from './application';
import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/intention';

export default class IntentionSerializer extends Serializer {
  @service('encoder') encoder;

  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;

  init() {
    super.init(...arguments);
    this.uri = this.encoder.uriTag();
  }

  ensureID(item) {
    if (!get(item, 'ID.length')) {
      item.Legacy = false;
    } else {
      item.Legacy = true;
      item.LegacyID = item.ID;
    }

    if (item.SourcePeer) {
      item.ID = this
        .uri`peer:${item.SourcePeer}:${item.SourceNS}:${item.SourceName}:${item.DestinationPartition}:${item.DestinationNS}:${item.DestinationName}`;
    } else {
      item.ID = this
        .uri`${item.SourcePartition}:${item.SourceNS}:${item.SourceName}:${item.DestinationPartition}:${item.DestinationNS}:${item.DestinationName}`;
    }

    return item;
  }

  respondForQuery(respond, query) {
    return super.respondForQuery(
      (cb) =>
        respond((headers, body) => {
          return cb(
            headers,
            body.map((item) => this.ensureID(item))
          );
        }),
      query
    );
  }

  respondForQueryRecord(respond, query) {
    return super.respondForQueryRecord(
      (cb) =>
        respond((headers, body) => {
          body = this.ensureID(body);
          return cb(headers, body);
        }),
      query
    );
  }

  respondForCreateRecord(respond, serialized, data) {
    const slugKey = this.slugKey;
    const primaryKey = this.primaryKey;
    return respond((headers, body) => {
      body = data;
      body.ID = this
        .uri`${serialized.SourcePartition}:${serialized.SourceNS}:${serialized.SourceName}:${serialized.DestinationPartition}:${serialized.DestinationNS}:${serialized.DestinationName}`;
      return this.fingerprint(primaryKey, slugKey, body.Datacenter)(body);
    });
  }

  respondForUpdateRecord(respond, serialized, data) {
    const slugKey = this.slugKey;
    const primaryKey = this.primaryKey;
    return respond((headers, body) => {
      body = data;
      body.LegacyID = body.ID;
      body.ID = serialized.ID;
      return this.fingerprint(primaryKey, slugKey, body.Datacenter)(body);
    });
  }
}
