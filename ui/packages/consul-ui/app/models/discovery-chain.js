import Model, { attr } from '@ember-data/model';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ServiceName';

export default class DiscoveryChain extends Model {
  @attr('string') uid;
  @attr('string') ServiceName;

  @attr('string') Datacenter;
  @attr() Chain; // {}
  @attr() meta; // {}
}
