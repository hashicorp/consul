import Service from '@ember/service';
import atob from 'consul-ui/utils/atob';
import btoa from 'consul-ui/utils/btoa';

export default class EncoderService extends Service {
  uriComponent = encodeURIComponent;

  atob() {
    return atob(...arguments);
  }

  btoa() {
    return btoa(...arguments);
  }

  uriJoin() {
    return this.joiner(this.uriComponent, '/', '')(...arguments);
  }

  uriTag() {
    return this.tag(this.uriJoin.bind(this));
  }

  joiner = (encoder, joiner = '', defaultValue = '') => (values, strs) =>
    (strs || Array(values.length).fill(joiner)).reduce(
      (prev, item, i) => `${prev}${item}${encoder(values[i] || defaultValue)}`,
      ''
    );

  tag(join) {
    return (strs, ...values) => join(values, strs);
  }
}
