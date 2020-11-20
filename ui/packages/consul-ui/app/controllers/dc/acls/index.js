import { action } from '@ember/object';
import Controller from '@ember/controller';
export default class IndexController extends Controller {
  @action
  sendClone(item) {
    this.send('clone', item);
  }
}
