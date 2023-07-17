import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class PeerGenerateFieldSets extends Component {
  @action
  onInput(addresses) {
    if (addresses) {
      addresses = addresses.split(',').map(address => address.trim());
    } else {
      addresses = [];
    }

    this.args.item.ServerExternalAddresses = addresses;
  } 
}
