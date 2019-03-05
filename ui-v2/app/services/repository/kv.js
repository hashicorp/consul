import RepositoryService from 'consul-ui/services/repository';
import { Promise } from 'rsvp';
import isFolder from 'consul-ui/utils/isFolder';
import { get, set } from '@ember/object';
import { PRIMARY_KEY } from 'consul-ui/models/kv';

const modelName = 'kv';
export default RepositoryService.extend({
  getModelName: function() {
    return modelName;
  },
  getPrimaryKey: function() {
    return PRIMARY_KEY;
  },
  // this one gives you the full object so key,values and meta
  findBySlug: function(key, dc) {
    if (isFolder(key)) {
      const id = JSON.stringify([dc, key]);
      let item = get(this, 'store').peekRecord(this.getModelName(), id);
      if (!item) {
        item = this.create();
        set(item, 'Key', key);
        set(item, 'Datacenter', dc);
      }
      return Promise.resolve(item);
    }
    return get(this, 'store').queryRecord(this.getModelName(), {
      id: key,
      dc: dc,
    });
  },
  // this one only gives you keys
  // https://www.consul.io/api/kv.html
  findAllBySlug: function(key, dc) {
    if (key === '/') {
      key = '';
    }
    return this.get('store')
      .query(this.getModelName(), {
        id: key,
        dc: dc,
        separator: '/',
      })
      .then(function(items) {
        return items.filter(function(item) {
          return key !== get(item, 'Key');
        });
      })
      .catch(e => {
        if (e.errors && e.errors[0] && e.errors[0].status == '404') {
          const id = JSON.stringify([dc, key]);
          const record = get(this, 'store').peekRecord(this.getModelName(), id);
          if (record) {
            record.destroyRecord();
          }
        }
        throw e;
      });
  },
});
