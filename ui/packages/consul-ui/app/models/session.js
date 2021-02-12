import Model, { attr } from '@ember-data/model';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';

export default class Session extends Model {
  @attr('string') uid;
  @attr('string') ID;

  @attr('string') Name;
  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Node;
  @attr('string') Behavior;
  @attr('string') TTL;
  @attr('number') LockDelay;
  @attr('number') SyncTime;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;

  @attr({ defaultValue: () => [] }) Checks;
  @attr({ defaultValue: () => [] }) Resources; // []
}
