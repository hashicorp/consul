import { helper } from '@ember/component/helper';
import { get } from '@ember/object';

export function serviceExternalSource(params, hash) {
  let source = get(params[0], 'ExternalSources.firstObject');
  if (!source) {
    source = get(params[0], 'Meta.external-source');
  }
  const prefix = typeof hash.prefix === 'undefined' ? '' : hash.prefix;
  if (source) {
    return `${prefix}${source}`;
  }
  return;
}

export default helper(serviceExternalSource);
