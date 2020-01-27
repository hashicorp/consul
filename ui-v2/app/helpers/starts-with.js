import { helper } from '@ember/component/helper';

export function startsWith([needle, haystack = ''] /*, hash*/) {
  return haystack.startsWith(needle);
}

export default helper(startsWith);
