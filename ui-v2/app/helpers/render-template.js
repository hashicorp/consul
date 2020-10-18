import { helper } from '@ember/component/helper';
import { get } from '@ember/object';

// Covers alpha-capitalized dot separated API keys such as
// `{{Name}}`, `{{Service.Name}}` etc. but not `{{}}`
const templateRe = /{{([A-Za-z.0-9_-]+)}}/g;
export default helper(function renderTemplate([template, vars]) {
  if (typeof vars !== 'undefined' && typeof template !== 'undefined') {
    return template.replace(templateRe, function(match, group) {
      try {
        return encodeURIComponent(get(vars, group) || '');
      } catch (e) {
        return '';
      }
    });
  }
  return '';
});
