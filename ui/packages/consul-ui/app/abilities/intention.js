import BaseAbility from './base';

export default class IntentionAbility extends BaseAbility {
  resource = 'intention';

  get canWrite() {
    return super.canWrite && (typeof this.item === 'undefined' || !this.canViewCRD);
  }
  get canViewCRD() {
    return (typeof this.item !== 'undefined' && this.item.IsManagedByCRD);
  }
}
