import { inject as service } from '@ember/service';
import { Ability } from 'ember-can';

const ACCESS_READ = 'read';
const ACCESS_WRITE = 'write';
const ACCESS_LIST = 'list';

export default class BaseAbility extends Ability {
  @service('repository/permission') permissions;

  resource = '';

  generate(action) {
    return this.permissions.generate(this.resource, action);
  }

  get canRead() {
    return this.permissions.has(this.generate(ACCESS_READ));
  }

  get canList() {
    return this.canRead;
  }

  get canWrite() {
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
