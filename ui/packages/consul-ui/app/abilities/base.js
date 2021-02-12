import { inject as service } from '@ember/service';
import { get } from '@ember/object';
import { Ability } from 'ember-can';

export const ACCESS_READ = 'read';
export const ACCESS_WRITE = 'write';
export const ACCESS_LIST = 'list';

export default class BaseAbility extends Ability {
  @service('repository/permission') permissions;

  resource = '';

  generate(action) {
    return this.permissions.generate(this.resource, action);
  }

  generateForSegment(segment) {
    return [
      this.permissions.generate(this.resource, ACCESS_READ, segment),
      this.permissions.generate(this.resource, ACCESS_WRITE, segment),
    ];
  }

  get canRead() {
    if (typeof this.item !== 'undefined') {
      const perm = (get(this, 'item.Resources') || []).find(item => item.Access === ACCESS_READ);
      if (perm) {
        return perm.Allow;
      }
    }
    return this.permissions.has(this.generate(ACCESS_READ));
  }

  get canList() {
    if (typeof this.item !== 'undefined') {
      const perm = (get(this, 'item.Resources') || []).find(item => item.Access === ACCESS_LIST);
      if (perm) {
        return perm.Allow;
      }
    }
    return this.permissions.has(this.generate(ACCESS_LIST));
  }

  get canWrite() {
    if (typeof this.item !== 'undefined') {
      const perm = (get(this, 'item.Resources') || []).find(item => item.Access === ACCESS_WRITE);
      if (perm) {
        return perm.Allow;
      }
    }
    return this.permissions.has(this.generate(ACCESS_WRITE));
  }

  get canCreate() {
    return this.canWrite;
  }

  get canDelete() {
    return this.canWrite;
  }

  get canUpdate() {
    return this.canWrite;
  }
}
