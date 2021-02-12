import RepositoryService from 'consul-ui/services/repository';
import isFolder from 'consul-ui/utils/isFolder';
import { get } from '@ember/object';
import { PRIMARY_KEY } from 'consul-ui/models/kv';
import { ACCESS_LIST } from 'consul-ui/abilities/base';

const modelName = 'kv';
export default class KvService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  // this one gives you the full object so key,values and meta
  async findBySlug(slug, dc, nspace, configuration = {}) {
    if (isFolder(slug)) {
      // we only use findBySlug for a folder when we are looking to create a
      // parent for a key for retriveing something Model shaped. Therefore we
      // only use existing records or a fake record with the correct Key,
      // which means we don't need to inpsect permissions as its an already
      // existing KV or a fake one

      // TODO: This very much shouldn't be here,
      // needs to eventually use ember-datas generateId thing
      // in the meantime at least our fingerprinter
      const id = JSON.stringify([nspace, dc, slug]);
      let item = this.store.peekRecord(this.getModelName(), id);
      if (!item) {
        item = this.create({
          Key: slug,
          Datacenter: dc,
          Namespace: nspace,
        });
      }
      return item;
    } else {
      return super.findBySlug(slug, dc, nspace, configuration);
    }
  }

  // this one only gives you keys
  // https://www.consul.io/api/kv.html
  findAllBySlug(key, dc, nspace, configuration = {}) {
    if (key === '/') {
      key = '';
    }
    return this.authorizeBySlug(
      async () => {
        const query = {
          id: key,
          dc: dc,
          ns: nspace,
          separator: '/',
        };
        if (typeof configuration.cursor !== 'undefined') {
          query.index = configuration.cursor;
        }
        let items;
        try {
          items = await this.store.query(this.getModelName(), query);
        } catch (e) {
          if (get(e, 'errors.firstObject.status') === '404') {
            // TODO: This very much shouldn't be here,
            // needs to eventually use ember-datas generateId thing
            // in the meantime at least our fingerprinter
            const id = JSON.stringify([dc, key]);
            const record = this.store.peekRecord(this.getModelName(), id);
            if (record) {
              record.unloadRecord();
            }
          }
          throw e;
        }
        return items.filter(item => key !== get(item, 'Key'));
      },
      ACCESS_LIST,
      key,
      dc,
      nspace
    );
  }
}
