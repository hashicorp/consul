import { clickable } from 'ember-cli-page-object';
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
page.navigation.dc = clickable('[data-test-toggle-button="datacenters"]');
export default page;
