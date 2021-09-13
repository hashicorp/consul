import Service, { inject as service } from '@ember/service';
import { assert } from '@ember/debug';
import { typeOf } from '@ember/utils';
import { get, set } from '@ember/object';
import { isChangeset } from 'validated-changeset';
import HTTPError from 'consul-ui/utils/http/error';
import { ACCESS_READ } from 'consul-ui/abilities/base';

export default class RepositoryService extends Service {
  @service('store') store;
  @service('repository/permission') permissions;

  getModelName() {
    assert('RepositoryService.getModelName should be overridden', false);
  }

  getPrimaryKey() {
    assert('RepositoryService.getPrimaryKey should be overridden', false);
  }

  getSlugKey() {
    assert('RepositoryService.getSlugKey should be overridden', false);
  }

  /**
   * Creates a set of permissions based on an id/slug, loads in the access
   * permissions for them and checks/validates
   */
  async authorizeBySlug(cb, access, params) {
    params.resources = await this.permissions.findBySlug(params, this.getModelName());
    return this.validatePermissions(cb, access, params);
  }

  /**
   * Loads in the access permissions and checks/validates them for a set of
   * permissions
   */
  async authorizeByPermissions(cb, access, params) {
    params.resources = await this.permissions.authorize(params);
    return this.validatePermissions(cb, access, params);
  }

  /**
   * Checks already loaded permissions for certain access before calling cb to
   * return the thing you wanted to check the permissions on
   */
  async validatePermissions(cb, access, params) {
    // inspect the permissions for this segment/slug remotely, if we have zero
    // permissions fire a fake 403 so we don't even request the model/resource
    if (params.resources.length > 0) {
      const resource = params.resources.find(item => item.Access === access);
      if (resource && resource.Allow === false) {
        // TODO: Here we temporarily make a hybrid HTTPError/ember-data HTTP error
        // we should eventually use HTTPError's everywhere
        const e = new HTTPError(403);
        e.errors = [{ status: '403' }];
        throw e;
      }
    }
    const item = await cb();
    // add the `Resource` information to the record/model so we can inspect
    // them in other places like templates etc
    if (get(item, 'Resources')) {
      set(item, 'Resources', params.resources);
    }
    return item;
  }

  reconcile(meta = {}) {
    // unload anything older than our current sync date/time
    if (typeof meta.date !== 'undefined') {
      const checkNspace = meta.nspace !== '';
      const checkPartition = meta.partition !== '';
      this.store.peekAll(this.getModelName()).forEach(item => {
        const dc = get(item, 'Datacenter');
        if (dc === meta.dc) {
          if (checkNspace) {
            const nspace = get(item, 'Namespace');
            if (typeof nspace !== 'undefined' && nspace !== meta.nspace) {
              return;
            }
          }
          if (checkPartition) {
            const partition = get(item, 'Partition');
            if (typeof partiton !== 'undefined' && partition !== meta.partition) {
              return;
            }
          }
          const date = get(item, 'SyncTime');
          if (!item.isDeleted && typeof date !== 'undefined' && date != meta.date) {
            this.store.unloadRecord(item);
          }
        }
      });
    }
  }

  peekOne(id) {
    return this.store.peekRecord(this.getModelName(), id);
  }

  // @deprecated
  findAllByDatacenter(params, configuration = {}) {
    return this.findAll(...arguments);
  }

  findAll(params = {}, configuration = {}) {
    if (typeof configuration.cursor !== 'undefined') {
      params.index = configuration.cursor;
      params.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), params);
  }

  async findBySlug(params, configuration = {}) {
    if (params.id === '') {
      return this.create({
        Datacenter: params.dc,
        Namespace: params.ns,
        Partition: params.partition,
      });
    }
    if (typeof configuration.cursor !== 'undefined') {
      params.index = configuration.cursor;
      params.uri = configuration.uri;
    }
    return this.authorizeBySlug(
      () => this.store.queryRecord(this.getModelName(), params),
      ACCESS_READ,
      params
    );
  }

  create(obj) {
    // TODO: This should probably return a Promise
    return this.store.createRecord(this.getModelName(), obj);
  }

  persist(item) {
    // workaround for saving changesets that contain fragments
    // firstly commit the changes down onto the object if
    // its a changeset, then save as a normal object
    if (isChangeset(item)) {
      item.execute();
      item = item.data;
    }
    return item.save();
  }

  remove(obj) {
    let item = obj;
    if (typeof obj.destroyRecord === 'undefined') {
      item = obj.get('data');
    }
    // TODO: Change this to use vanilla JS
    // I think this was originally looking for a plain object
    // as opposed to an ember one
    if (typeOf(item) === 'object') {
      item = this.store.peekRecord(this.getModelName(), item[this.getPrimaryKey()]);
    }
    return item.destroyRecord().then(item => {
      return this.store.unloadRecord(item);
    });
  }

  invalidate() {
    // TODO: This should probably return a Promise
    this.store.unloadAll(this.getModelName());
  }
}
