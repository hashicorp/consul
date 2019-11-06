import { helper } from '@ember/component/helper';

export default helper(function routeMatch([params] /*, hash*/) {
  const keys = Object.keys(params);
  switch (true) {
    case keys.includes('Present'):
      return `${params.Invert ? `NOT ` : ``}present`;
    case keys.includes('Exact'):
      return `${params.Invert ? `NOT ` : ``}exactly matching "${params.Exact}"`;
    case keys.includes('Prefix'):
      return `${params.Invert ? `NOT ` : ``}prefixed by "${params.Prefix}"`;
    case keys.includes('Suffix'):
      return `${params.Invert ? `NOT ` : ``}suffixed by "${params.Suffix}"`;
    case keys.includes('Regex'):
      return `${params.Invert ? `NOT ` : ``}matching the regex "${params.Regex}"`;
  }
  return '';
});
