import Service from '@ember/service';
import atob from 'consul-ui/utils/atob';
import btoa from 'consul-ui/utils/btoa';

export default Service.extend({
  uriComponent: encodeURIComponent,
  atob: function() {
    return atob(...arguments);
  },
  btoa: function() {
    return btoa(...arguments);
  },
  uriJoin: function() {
    return this.joiner(this.uriComponent, '/', '')(...arguments);
  },
  uriTag: function() {
    return this.tag(this.uriJoin.bind(this));
  },
  joiner: (encoder, joiner = '', defaultValue = '') => (values, strs) =>
    (strs || Array(values.length).fill(joiner)).reduce(
      (prev, item, i) => `${prev}${item}${encoder(values[i] || defaultValue)}`,
      ''
    ),
  tag: function(join) {
    return (strs, ...values) => join(values, strs);
  },
});
