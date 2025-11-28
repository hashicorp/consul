import Controller from '@ember/controller';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';

export default class InstancesController extends Controller {
  @tracked proxies = [];

  @action setProxies(data) {
    this.proxies = Array.isArray(data) ? data : data ? [data] : [];
  }
}
