import { clickable, is } from 'ember-cli-page-object';
const page = {
  navigation: ['services', 'nodes', 'kvs', 'acls', 'intentions', 'docs', 'settings'].reduce(
    function(prev, item, i, arr) {
      const key = item;
      return Object.assign({}, prev, {
        [key]: clickable(`[data-test-main-nav-${item}] a`),
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
};
page.navigation.dc = clickable('[data-test-datacenter-menu] button');
page.navigation.nspace = clickable('[data-test-nspace-menu] button');
page.navigation.manageNspaces = clickable('[data-test-main-nav-nspaces] a');
page.navigation.manageNspacesIsVisible = is(
  ':checked',
  '[data-test-nspace-menu] > input[type="checkbox"]'
);
export default page;
