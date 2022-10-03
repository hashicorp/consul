import Service from '@ember/service';
import { get } from '@ember/object';
import { runInDebug } from '@ember/debug';
import atob from 'consul-ui/utils/atob';
import btoa from 'consul-ui/utils/btoa';

const createRegExpEncoder = function (re, encoder = (str) => str, strict = true) {
  return (template = '', vars = {}) => {
    if (template !== '') {
      return template.replace(re, (match, group) => {
        const value = get(vars, group);
        runInDebug(() => {
          if (strict && typeof value === 'undefined') {
            console.error(new Error(`${group} is undefined in ${template}`));
          }
        });
        return encoder(value || '');
      });
    }
    return '';
  };
};
export default class EncoderService extends Service {
  uriComponent = encodeURIComponent;

  createRegExpEncoder(re, encoder) {
    return createRegExpEncoder(re, encoder);
  }

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

  joiner =
    (encoder, joiner = '', defaultValue = '') =>
    (values, strs) =>
      (strs || Array(values.length).fill(joiner)).reduce(
        (prev, item, i) => `${prev}${item}${encoder(values[i] || defaultValue)}`,
        ''
      );

  tag(join) {
    return (strs, ...values) => join(values, strs);
  }
}
