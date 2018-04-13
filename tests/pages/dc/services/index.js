import { create, visitable, collection, text } from 'ember-cli-page-object';

import filter from 'consul-ui/tests/pages/components/catalog-filter';

export default create({
  visit: visitable('/:dc/services'),
  services: collection('[data-test-service]', {
    name: text('a'),
  }),
  dcs: collection('[data-test-datacenter-picker]'),

  filter: filter,
});
