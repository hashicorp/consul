export default (clickable, attribute, is, authForm) => scope => {
  const page = {
    navigation: [
      'services',
      'nodes',
      'kvs',
      'acls',
      'intentions',
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
    error: {
      status: attribute('data-test-status', '[data-test-status]'),
    },
  };
  page.navigation.login = clickable('[data-test-main-nav-auth] label');
  page.navigation.dc = clickable('[data-test-datacenter-menu] button');
  page.navigation.nspace = clickable('[data-test-nspace-menu] button');
  page.navigation.manageNspaces = clickable('[data-test-main-nav-nspaces] a');
  page.navigation.manageNspacesIsVisible = is(
    ':checked',
    '[data-test-nspace-menu] > input[type="checkbox"]'
  );
  return page;
};
