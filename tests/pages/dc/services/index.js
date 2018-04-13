import { create, visitable, collection, attribute, clickable } from 'ember-cli-page-object';

import page from 'consul-ui/tests/pages/components/page';
import filter from 'consul-ui/tests/pages/components/catalog-filter';

export default create({
  visit: visitable('/:dc/services'),
  services: collection('[data-test-service]', {
    name: attribute('data-test-service'),
    service: clickable('a'),
  }),
  dcs: collection('[data-test-datacenter-picker]'),
  navigation: page.navigation,

  filter: filter,
});
