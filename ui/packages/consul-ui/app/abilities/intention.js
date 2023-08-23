import BaseAbility from './base';

export default class IntentionAbility extends BaseAbility {
  resource = 'intention';

  get canWrite() {
    // Peered intentions aren't writable
    if(typeof this.item !== 'undefined' && typeof this.item.SourcePeer !== 'undefined') {
      return false;
    }
    return super.canWrite &&
      (typeof this.item === 'undefined' || !this.canViewCRD);
  }
  get canViewCRD() {
    return (typeof this.item !== 'undefined' && this.item.IsManagedByCRD);
  }
}
