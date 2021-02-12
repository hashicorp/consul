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

  reconcile(meta = {}) {
    // unload anything older than our current sync date/time
    if (typeof meta.date !== 'undefined') {
      const checkNspace = meta.nspace !== '';
      this.store.peekAll(this.getModelName()).forEach(item => {
        const dc = get(item, 'Datacenter');
        if (dc === meta.dc) {
          if (checkNspace) {
            const nspace = get(item, 'Namespace');
            if (typeof nspace !== 'undefined' && nspace !== meta.nspace) {
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

  findAllByDatacenter(dc, nspace, configuration = {}) {
    const query = {
      dc: dc,
      ns: nspace,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
      query.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), query);
  }

  async authorizeBySlug(cb, access, slug, dc, nspace) {
    return this.validatePermissions(
      cb,
      await this.permissions.findBySlug(slug, this.getModelName(), dc, nspace),
      access,
      dc,
      nspace
    );
  }

  async authorizeByPermissions(cb, permissions, access, dc, nspace) {
    return this.validatePermissions(
      cb,
      await this.permissions.authorize(permissions, dc, nspace),
      access,
      dc,
      nspace
    );
  }

  async validatePermissions(cb, permissions, access, dc, nspace) {
    // inspect the permissions for this segment/slug remotely, if we have zero
    // permissions fire a fake 403 so we don't even request the model/resource
    console.log(permissions);
    if (permissions.length > 0) {
      const permission = permissions.find(item => item.Access === access);
      if (permission && permission.Allow === false) {
        throw new HTTPError(403);
      }
    }
    const item = await cb();
    // add the `Resource` information to the record/model so we can inspect
    // them in other places like templates etc
    if (get(item, 'Resources')) {
      set(item, 'Resources', permissions);
    }
    return item;
  }

  async findBySlug(slug, dc, nspace, configuration = {}) {
    const query = {
      dc: dc,
      ns: nspace,
      id: slug,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
      query.uri = configuration.uri;
    }
    return this.authorizeBySlug(
      () => this.store.queryRecord(this.getModelName(), query),
      ACCESS_READ,
      slug,
      dc,
      nspace
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
