export default (collection, clickable, attribute, property, authForm, emptyState) => (scope) => {
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
      function (prev, item, i, arr) {
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
      function (prev, item, i, arr) {
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
  page.navigation.authMenu = clickable('[data-test-auth-menu]');
  page.navigation.login = clickable('[data-test-auth-menu-login]');
  page.navigation.dc = clickable('[data-test-datacenter-menu] button');
  page.navigation.nspace = clickable('[data-test-nspace-menu] button');
  page.navigation.partition = clickable('[data-test-partition-menu] button');
  page.navigation.manageNspaces = clickable(
    '[data-test-nspace-menu] [data-test-nav-selector-footer-link]'
  );
  page.navigation.manageNspacesIsVisible = property(
    ':checked',
    '[data-test-nspace-menu] > input[type="checkbox"]'
  );
  page.navigation.managePartitions = clickable(
    '[data-test-partition-menu] [data-test-nav-selector-footer-link]'
  );
  page.navigation.dcs = collection('[data-test-datacenter-menu] [data-test-dc-item]', {
    name: clickable(),
  });
  page.navigation.partitions = collection('[data-test-partition-menu] [data-test-partition-item]', {
    name: clickable(),
  });
  return page;
};
