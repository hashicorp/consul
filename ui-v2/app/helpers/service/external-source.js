import { helper } from '@ember/component/helper';
import { get } from '@ember/object';

export function serviceExternalSource(params, hash) {
  const source = get(params[0], 'Meta.external-source');
  const prefix = typeof hash.prefix === 'undefined' ? '' : hash.prefix;
  if (source) {
    return `${prefix}${source}`;
  }
  return;
}

export default helper(serviceExternalSource);
