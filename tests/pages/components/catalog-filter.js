import { is, clickable, triggerable } from 'ember-cli-page-object';
export default ['', 'passing', 'warning', 'critical'].reduce(
  function(prev, item, i, arr) {
    const key = item === '' ? 'all' : item;
    return Object.assign({}, prev, {
      [`${key}IsSelected`]: is(':checked', `[data-test-radiobutton="status_${item}"] input`),
      [key]: clickable(`[data-test-radiobutton="status_${item}"]`),
    });
  },
  {
    scope: '[data-test-catalog-filter]',
    search: triggerable('keypress', '[name="s"]'),
  }
);
