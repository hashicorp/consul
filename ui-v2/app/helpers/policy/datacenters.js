import { helper } from '@ember/component/helper';
import { get } from '@ember/object';

export function policyDatacenters(params, hash) {
  if (typeof get(params[0], 'Datacenters') === 'undefined') {
    return [hash['global'] || 'All'];
  }
  return get(params[0], 'Datacenters');
}

export default helper(policyDatacenters);
