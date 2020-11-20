import Model, { attr } from '@ember-data/model';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';

export default class Acl extends Model {
  @attr('string') uid;
  @attr('string') ID;

  @attr('string') Datacenter;
  // TODO: Why didn't I have to do this for KV's? This is to ensure that Name
  // is '' and not null when creating maybe its due to the fact that `Key` is
  // the primaryKey in Kv's
  @attr('string', { defaultValue: () => '' }) Name;
  @attr('string') Type;
  @attr('string') Rules;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;
}
