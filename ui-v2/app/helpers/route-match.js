import { helper } from '@ember/component/helper';

export default helper(function routeMatch([item], hash) {
  const keys = Object.keys(item.data || item);
  switch (true) {
    case keys.includes('Present'):
      return `${item.Invert ? `NOT ` : ``}present`;
    case keys.includes('Exact'):
      return `${item.Invert ? `NOT ` : ``}exactly matching "${item.Exact}"`;
    case keys.includes('Prefix'):
      return `${item.Invert ? `NOT ` : ``}prefixed by "${item.Prefix}"`;
    case keys.includes('Suffix'):
      return `${item.Invert ? `NOT ` : ``}suffixed by "${item.Suffix}"`;
    case keys.includes('Regex'):
      return `${item.Invert ? `NOT ` : ``}matching the regex "${item.Regex}"`;
  }
  return '';
});
