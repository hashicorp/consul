import Service, { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import { typeOf } from '@ember/utils';
// clone: function(acl, dc) {
//   const slug = acl.get('ID');
//   const newAcl = this.create();
//   newAcl.set('ID', slug);
//   return newAcl.save().then(
//     (acl) => {
//       return this.findBySlug(acl.get('ID'), dc).then(
//         (acl) => {
//           this.get('store').pushPayload(
//             {
//               acls: acl.serialize()
//             }
//           );
//           return acl;
//         }
//       );
//     }
//   );
// },
export default Service.extend({
  store: service('store'),
  findAllByDatacenter: function(dc) {
    return get(this, 'store')
      .query('acl', {
        dc: dc,
      })
      .then(function(items) {
        // TODO: Sort with anonymous first
        return items.forEach(function(item, i, arr) {
          set(item, 'Datacenter', dc);
        });
      });
  },
  findBySlug: function(slug, dc) {
    return get(this, 'store')
      .queryRecord('acl', {
        acl: slug,
        dc: dc,
      })
      .then(function(item) {
        set(item, 'Datacenter', dc);
        return item;
      });
  },
  create: function() {
    return get(this, 'store').createRecord('acl');
  },
  persist: function(item) {
    return item.save();
  },
  remove: function(obj) {
    // TODO: check to see if this is still needed
    // seems like ember-changeset .get('data') still needs this
    //
    let item = obj;
    if (typeof obj.destroyRecord === 'undefined') {
      item = obj.get('data');
    }
    if (typeOf(item) === 'object') {
      const id = item.ID;
      const dc = item.Datacenter;
      // TODO: This won't work for multi dc?
      // id's need to be 'dc-id'
      item = get(this, 'store').peekRecord('acl', id);
      if (item == null) {
        item = this.create();
        set(item, 'ID', id);
        set(item, 'Datacenter', dc);
      }
    }
    return item.destroyRecord().then(item => {
      return get(this, 'store').unloadRecord(item);
    });
  },
});
