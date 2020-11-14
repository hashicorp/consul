import RepositoryService from 'consul-ui/services/repository';
import { get } from '@ember/object';
import { PRIMARY_KEY } from 'consul-ui/models/acl';
const modelName = 'acl';
export default class AclService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  clone(item) {
    return this.store.clone(this.getModelName(), get(item, this.getPrimaryKey()));
  }
}
