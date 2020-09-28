import { helper } from '@ember/component/helper';

export default helper(function routeMatch([item], hash) {
  const prop = ['Present', 'Exact', 'Prefix', 'Suffix', 'Regex'].find(
    prop => typeof item[prop] !== 'undefined'
  );

  switch (prop) {
    case 'Present':
      return `${item.Invert ? `NOT ` : ``}present`;
    case 'Exact':
      return `${item.Invert ? `NOT ` : ``}exactly matching "${item.Exact}"`;
    case 'Prefix':
      return `${item.Invert ? `NOT ` : ``}prefixed by "${item.Prefix}"`;
    case 'Suffix':
      return `${item.Invert ? `NOT ` : ``}suffixed by "${item.Suffix}"`;
    case 'Regex':
      return `${item.Invert ? `NOT ` : ``}matching the regex "${item.Regex}"`;
  }
  return '';
});
