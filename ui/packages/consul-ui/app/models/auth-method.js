import Model, { attr } from '@ember-data/model';
import { or } from '@ember/object/computed';
import parse from 'parse-duration';
import { computed } from '@ember/object';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Name';

export default class AuthMethod extends Model {
  @attr('string') uid;
  @attr('string') Name;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;
  @attr('string', { defaultValue: () => '' }) Description;
  @attr('string', { defaultValue: () => '' }) DisplayName;
  @attr('string', { defaultValue: () => 'local' }) TokenLocality;
  @attr('string') Type;
  @attr() NamespaceRules;
  @or('DisplayName', 'Name') MethodName;
  @attr() Config;
  @attr('string') MaxTokenTTL;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;
  @attr() Datacenters; // string[]
  @attr() meta; // {}

  @computed('MaxTokenTTL')
  get TokenTTL() {
    return parse(this.MaxTokenTTL);
  }
}
