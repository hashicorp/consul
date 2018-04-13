import { create, visitable, collection, text } from 'ember-cli-page-object';

import page from 'consul-ui/tests/pages/components/page';
import filter from 'consul-ui/tests/pages/components/catalog-filter';

export default create({
  visit: visitable('/:dc/services'),
  services: collection('[data-test-service]', {
    name: text('a'),
  }),
  dcs: collection('[data-test-datacenter-picker]'),
  navigation: page.navigation,

  filter: filter,
});
