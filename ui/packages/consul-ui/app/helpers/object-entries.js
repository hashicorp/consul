import { helper } from '@ember/component/helper';

export function objectEntries([obj = {}] /*, hash*/) {
  if (obj == null) {
    return [];
  }
  return Object.entries(obj);
}

export default helper(objectEntries);
