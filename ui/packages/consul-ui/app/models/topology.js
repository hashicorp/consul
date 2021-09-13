import Model, { attr } from '@ember-data/model';
import { computed } from '@ember/object';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ServiceName';

export default class Topology extends Model {
  @attr('string') uid;
  @attr('string') ServiceName;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;
  @attr('string') Protocol;
  @attr('boolean') FilteredByACLs;
  @attr('boolean') TransparentProxy;
  @attr('boolean') DefaultAllow;
  @attr('boolean') WildcardIntention;
  @attr() Upstreams; // Service[]
  @attr() Downstreams; // Service[],
  @attr() meta; // {}

  @computed('Downstreams')
  get notDefinedIntention() {
    let undefinedDownstream = false;

    undefinedDownstream =
      this.Downstreams.filter(
        item =>
          item.Source === 'specific-intention' && !item.TransparentProxy && item.Intention.Allowed
      ).length !== 0;

    return undefinedDownstream;
  }

  @computed('FilteredByACL', 'DefaultAllow', 'WildcardIntention', 'notDefinedIntention')
  get collapsible() {
    if (this.DefaultAllow && this.FilteredByACLs && this.notDefinedIntention) {
      return true;
    } else if (this.WildcardIntention && this.FilteredByACLs && this.notDefinedIntention) {
      return true;
    }

    return false;
  }
}
