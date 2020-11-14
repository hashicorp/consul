import Model, { attr } from '@ember-data/model';
import { computed } from '@ember/object';
import isFolder from 'consul-ui/utils/isFolder';

export const PRIMARY_KEY = 'uid';
// not really a slug as it contains slashes but all intents and purposes its
// my 'slug'
export const SLUG_KEY = 'Key';

export default class Kv extends Model {
  @attr('string') uid;
  @attr('string') Key;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('number') LockIndex;
  @attr('number') Flags;
  // TODO: Consider defaulting all strings to '' because `typeof null !==
  // 'string'` look into what other transformers do with `null` also
  // preferably removeNull would be done in this layer also as if a property
  // is `null` default Values don't kick in, which also explains `Tags`
  // elsewhere
  @attr('string') Value; //, {defaultValue: function() {return '';}}
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;
  @attr('string') Session;

  @computed('Key')
  get isFolder() {
    return isFolder(this.Key || '');
  }
}
