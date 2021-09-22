import BaseAbility, { ACCESS_LIST } from './base';

export default class KVAbility extends BaseAbility {
  resource = 'key';

  generateForSegment(segment) {
    let resources = super.generateForSegment(segment);
    if (segment.endsWith('/')) {
      resources = resources.concat(this.permissions.generate(this.resource, ACCESS_LIST, segment));
    }
    return resources;
  }
  get canRead() {
    return true;
  }

  get canList() {
    return true;
  }

  get canWrite() {
    return true;
  }
}
