import { create, visitable, collection, attribute } from 'ember-cli-page-object';
import filter from 'consul-ui/tests/pages/components/catalog-filter';

export default create({
  visit: visitable('/:dc/services/:service'),
  nodes: collection('[data-test-node]', {
    name: attribute('data-test-node'),
  }),
  filter: filter,
});
