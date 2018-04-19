import Service, { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
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
// findAllByDatacenter: function(dc) {
//   return get('/v1/acl/list', dc).then(function(data) {
//     const objs = [];
//     data.map(function(obj) {
//       if (obj.ID === 'anonymous') {
//         objs.unshift(Entity.create(obj));
//       } else {
//         objs.push(Entity.create(obj));
//       }
//     });
//     return objs;
//   });
// },
export default Service.extend({
  store: service('store'),
  findAllByDatacenter: function(dc) {
    return get(this, 'store')
      .query('acl', {
        dc: dc,
      })
      .then(function(items) {
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
  remove: function(item) {
    return item.destroyRecord().then(item => {
      // really?
      return get(this, 'store').unloadRecord(item);
    });
  },
});
