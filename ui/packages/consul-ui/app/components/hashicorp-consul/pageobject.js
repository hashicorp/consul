export default (collection, clickable, attribute, is, authForm, emptyState) => scope => {
  const page = {
    navigation: [
      'services',
      'nodes',
      'kvs',
      'intentions',
      'tokens',
      'policies',
      'roles',
      'help',
      'settings',
      'auth',
    ].reduce(
      function(prev, item, i, arr) {
        const key = item;
        return Object.assign({}, prev, {
          [key]: clickable(`[data-test-main-nav-${item}] > *`),
        });
      },
      {
        scope: '[data-test-navigation]',
      }
    ),
    footer: ['copyright', 'docs'].reduce(
      function(prev, item, i, arr) {
        const key = item;
        return Object.assign({}, prev, {
          [key]: clickable(`[data-test-main-nav-${item}`),
        });
      },
      {
        scope: '[data-test-footer]',
      }
    ),
    authdialog: {
      form: authForm(),
    },
    emptystate: emptyState(),
    // TODO: errors aren't strictly part of this component
    error: {
      status: attribute('data-test-status', '[data-test-status]'),
    },
  };
  page.navigation.login = clickable('[data-test-main-nav-auth] button');
  page.navigation.dc = clickable('[data-test-datacenter-menu] button');
  page.navigation.nspace = clickable('[data-test-nspace-menu] button');
  page.navigation.manageNspaces = clickable('[data-test-main-nav-nspaces] a');
  page.navigation.manageNspacesIsVisible = is(
    ':checked',
    '[data-test-nspace-menu] > input[type="checkbox"]'
  );
  page.navigation.dcs = collection('[data-test-datacenter-menu] [data-test-dc-item]', {
    name: clickable('a'),
  });
  return page;
};
