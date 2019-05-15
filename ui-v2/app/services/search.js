import Service from '@ember/service';
export default Service.extend({
  searchable: function() {
    return {
      addEventListener: function() {},
      removeEventListener: function() {},
    };
  },
});
