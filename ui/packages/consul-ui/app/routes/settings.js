import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class SettingsRoute extends Route {
  @service('client/http') client;
  @service('settings') repo;

  @action
  change(slug, item) {
    switch (slug) {
      case 'client':
        item.blocking = !item.blocking;
        if (!item.blocking) {
          this.client.abort();
        }
        break;
    }
    this.repo.persist({
      [slug]: item,
    });
  }
}
