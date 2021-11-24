import { helper } from '@ember/component/helper';

export function collapsibleNotices(params, hash) {
  // This filter will only return truthy items
  const noticesCount = params.filter(Boolean).length;
  return noticesCount > 2;
}

export default helper(collapsibleNotices);
