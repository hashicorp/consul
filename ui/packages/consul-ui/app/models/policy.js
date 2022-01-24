import Model, { attr } from '@ember-data/model';
import { computed } from '@ember/object';

export const MANAGEMENT_ID = '00000000-0000-0000-0000-000000000001';
export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';

export default class Policy extends Model {
  @attr('string') uid;
  @attr('string') ID;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;
  @attr('string', { defaultValue: () => '' }) Name;
  @attr('string', { defaultValue: () => '' }) Description;
  @attr('string', { defaultValue: () => '' }) Rules;
  @attr('number') SyncTime;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;
  @attr() Datacenters; // string[]
  @attr() meta; // {}
  // frontend only for templated policies (Identities)
  @attr('string', { defaultValue: () => '' }) template;
  // frontend only for ordering where CreateIndex can't be used
  @attr('number', { defaultValue: () => new Date().getTime() }) CreateTime;

  @computed('ID')
  get isGlobalManagement() {
    return this.ID === MANAGEMENT_ID;
  }
}
