import Service from '@ember/service';
import btoa from 'consul-ui/utils/btoa';
export default class BtoaService extends Service {
  execute() {
    return btoa(...arguments);
  }
}
