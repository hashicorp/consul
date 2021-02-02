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

  get canCreate() {
    return this.permissions.has(this.generate(ACCESS_WRITE));
  }

  get canDelete() {
    return this.permissions.has(this.generate(ACCESS_WRITE));
  }

  get canRead() {
    return this.permissions.has(this.generate(ACCESS_READ));
  }

  get canList() {
    return this.permissions.has(this.generate(ACCESS_LIST));
  }

  get canUpdate() {
    return this.permissions.has(this.generate(ACCESS_WRITE));
  }
}
