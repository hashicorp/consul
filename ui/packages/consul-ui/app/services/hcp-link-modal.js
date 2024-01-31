import Service from '@ember/service';

export default class HcpLinkModal extends Service {
  isModalVisible = false;

  show() {
    this.isModalVisible = true;
  }

  hide() {
    this.isModalVisible = false;
  }
}
