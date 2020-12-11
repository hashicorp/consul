import { computed } from '@ember/object';
import Controller from '@ember/controller';

export default class IndexController extends Controller {
  @computed('items.[]')
  get services() {
    return this.items.filter(function(item) {
      return item.Kind !== 'connect-proxy';
    });
  }

  @computed('services')
  get externalSources() {
    const sources = this.services.reduce(function(prev, item) {
      return prev.concat(item.ExternalSources || []);
    }, []);
    // unique, non-empty values, alpha sort
    return [...new Set(sources)].filter(Boolean).sort();
  }
}
