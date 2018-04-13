import { is, clickable, triggerable } from 'ember-cli-page-object';
export default ['', 'management', 'client'].reduce(
  function(prev, item, i, arr) {
    const key = item === '' ? 'all' : item;
    return Object.assign({}, prev, {
      [`${key}IsSelected`]: is(':checked', `[data-test-radiobutton="type_${item}"] input`),
      [key]: clickable(`[data-test-radiobutton="type_${item}"]`),
    });
  },
  {
    scope: '[data-test-acl-filter]',
    search: triggerable('keypress', '[name="s"]'),
  }
);
