import Model, { attr } from '@ember-data/model';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Name';

export default class AuthMethod extends Model {
  @attr('string') uid;
  @attr('string') Name;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string', { defaultValue: () => '' }) Description;
  @attr('string', { defaultValue: () => '' }) DisplayName;
  @attr('string', { defaultValue: () => 'local' }) TokenLocality;
  @attr('string') Type;
  @attr('string') Host;
  @attr('string') ServiceAccountJWT;
  @attr('string') CACert;
  @attr('string') MaxTokenTTL;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;
  @attr() Datacenters; // string[]
  @attr() meta; // {}
}
