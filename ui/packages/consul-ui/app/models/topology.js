import Model, { attr } from '@ember-data/model';
import { computed } from '@ember/object';

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

  @computed('Upstreams', 'Downstreams')
  get undefinedIntention() {
    let undefinedUpstream = false;
    let undefinedDownstream = false;

    undefinedUpstream =
      this.Upstreams.filter(
        item =>
          item.Source === 'specific-intention' && !item.TransparentProxy && item.Intention.Allowed
      ).length !== 0;

    undefinedDownstream =
      this.Downstreams.filter(
        item =>
          item.Source === 'specific-intention' && !item.TransparentProxy && item.Intention.Allowed
      ).length !== 0;

    return undefinedUpstream || undefinedDownstream;
  }
}
