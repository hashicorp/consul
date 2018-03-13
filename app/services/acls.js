import Service, { inject as service } from '@ember/service';

import put from 'consul-ui/utils/request/put';
export default Service.extend({
  store: service('store'),
  findAllByDatacenter: function(dc) {
    return this.get('store').query('acl', {
      dc: dc,
      token: '',
    });
  },
  findBySlug: function(slug, dc) {
    return this.get('store').queryRecord('acl', {
      acl: slug,
      dc: dc,
      token: '',
    });
  },
  persist: function(acl, dc) {
    const token = '';
    return put('/v1/acl/update', dc, token, JSON.stringify(acl));
  },
  clone: function(acl, dc) {
    const token = '';
    return put('/v1/acl/clone/' + acl.ID, dc, token);
  },
  remove: function(acl, dc) {
    const token = '';
    return put('/v1/acl/destroy/' + acl.ID, dc, token);
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
