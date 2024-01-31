import Service from '@ember/service';
import { tracked } from '@glimmer/tracking';

export default class HcpLinkModalService extends Service {
  @tracked isModalVisible = false;

  show() {
    this.isModalVisible = true;
  }

  hide() {
    this.isModalVisible = false;
  }
}
