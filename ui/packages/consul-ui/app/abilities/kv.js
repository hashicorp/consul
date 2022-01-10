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
  /**/
  // Temporarily revert to pre-1.10 UI functionality by overwriting frontend
  // permissions. These are used to hide certain UI elements, but they are
  // still enforced on the backend.
  // This temporary measure should be removed again once https://github.com/hashicorp/consul/issues/11098
  // has been resolved
  get canRead() {
    return true;
  }

  get canList() {
    return true;
  }

  get canWrite() {
    return true;
  }
  /**/
}
