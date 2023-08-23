import Model, { attr } from '@ember-data/model';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';

export default class Role extends Model {
  @attr('string') uid;
  @attr('string') ID;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;
  @attr('string', { defaultValue: () => '' }) Name;
  @attr('string', { defaultValue: () => '' }) Description;
  @attr({ defaultValue: () => [] }) Policies;
  @attr({ defaultValue: () => [] }) ServiceIdentities;
  @attr({ defaultValue: () => [] }) NodeIdentities;
  @attr('number') SyncTime;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;
  // frontend only for ordering where CreateIndex can't be used i.e. for when
  // we need to order items that aren't yet saved to the backend, for example
  // in the role-selector
  @attr('number') CreateTime;
  // TODO: Figure out whether we need this or not
  @attr() Datacenters; // string[]
  @attr('string') Hash;
}
