import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { computed, get } from '@ember/object';
import isFolder from 'consul-ui/utils/isFolder';

export const PRIMARY_KEY = 'uid';
// not really a slug as it contains slashes but all intents and purposes
// its my 'slug'
export const SLUG_KEY = 'Key';

export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  LockIndex: attr('number'),
  Flags: attr('number'),
  // TODO: Consider defaulting all strings to '' because `typeof null !== 'string'`
  // look into what other transformers do with `null` also
  // preferably removeNull would be done in this layer also as if a property is `null`
  // default Values don't kick in, which also explains `Tags` elsewhere
  Value: attr('string'), //, {defaultValue: function() {return '';}}
  CreateIndex: attr('string'),
  ModifyIndex: attr('string'),
  Session: attr('string'),
  Datacenter: attr('string'),

  isFolder: computed('Key', function() {
    return isFolder(get(this, 'Key') || '');
  }),
});
