import { helper } from '@ember/component/helper';

export function isUndefined([value, type], hash) {
  // we have to check is destroyed here as .content is set to null
  // rather than undefined when a if/when a destryed ember value/object
  // is passed through
  return typeof value === 'undefined' || value.isDestroyed;
}

export default helper(isUndefined);
