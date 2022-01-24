import Model, { attr } from '@ember-data/model';

export const PRIMARY_KEY = 'uid';
export const FOREIGN_KEY = 'Datacenter';
export const SLUG_KEY = 'Name';

export default class Datacenter extends Model {
  @attr('string') uid;
  @attr('string') Name;
  @attr('boolean') Local;
  @attr('boolean') Primary;
  @attr('string') DefaultACLPolicy;

  @attr('boolean', { defaultValue: () => true }) MeshEnabled;
}
