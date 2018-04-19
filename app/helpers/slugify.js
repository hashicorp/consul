import { helper } from '@ember/component/helper';

// TODO: Improve this
export function slugify([str = ''], hash) {
  return str.replace(/ /g, '-').toLowerCase();
}

export default helper(slugify);
