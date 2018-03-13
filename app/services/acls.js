import Service, { inject as service } from '@ember/service';
import { assign } from '@ember/polyfills';
export default Service.extend({
  store: service('store'),
  findAllByDatacenter: function(dc) {
    return this.get('store').query('acl', {
      dc: dc,
    });
  },
  findBySlug: function(slug, dc) {
    return this.get('store').queryRecord('acl', {
      acl: slug,
      dc: dc,
    });
  },
  persist: function(acl, dc) {
    return acl.save();
  },
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
  remove: function(acl, dc) {
    return acl.destroyRecord();
  },
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
  create: function() {
    return this.get('store').createRecord('acl');
  },
});
