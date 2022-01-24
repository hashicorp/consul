import Model, { attr } from '@ember-data/model';
import { computed } from '@ember/object';
import isFolder from 'consul-ui/utils/isFolder';
import { nullValue } from 'consul-ui/decorators/replace';

export const PRIMARY_KEY = 'uid';
// not really a slug as it contains slashes but all intents and purposes its
// my 'slug'
export const SLUG_KEY = 'Key';

export default class Kv extends Model {
  @attr('string') uid;
  @attr('string') Key;

  @attr('number') SyncTime;
  @attr() meta; // {}

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;
  @attr('number') LockIndex;
  @attr('number') Flags;
  @nullValue(undefined) @attr('string') Value;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;
  @attr('string') Session;
  @attr({ defaultValue: () => [] }) Resources; // []

  @computed('isFolder')
  get Kind() {
    return this.isFolder ? 'folder' : 'key';
  }

  @computed('Key')
  get isFolder() {
    return isFolder(this.Key || '');
  }
}
