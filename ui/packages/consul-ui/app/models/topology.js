import Model, { attr } from '@ember-data/model';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ServiceName';

export default class Topology extends Model {
  @attr('string') uid;
  @attr('string') ServiceName;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Protocol;
  @attr('boolean') FilteredByACLs;
  @attr('boolean') TransparentProxy;
  @attr('boolean') DefaultAllow;
  @attr('boolean') WildcardIntention;
  @attr() Upstreams; // Service[]
  @attr() Downstreams; // Service[],
  @attr() meta; // {}
}
