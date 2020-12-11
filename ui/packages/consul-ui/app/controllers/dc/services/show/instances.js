import { computed } from '@ember/object';
import Controller from '@ember/controller';

export default class InstancesController extends Controller {
  @computed('items')
  get externalSources() {
    const sources = this.items.reduce(function(prev, item) {
      return prev.concat(item.ExternalSources || []);
    }, []);
    // unique, non-empty values, alpha sort
    return [...new Set(sources)].filter(Boolean).sort();
  }
}
