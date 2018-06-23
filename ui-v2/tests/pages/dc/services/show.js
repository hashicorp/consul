import { create, visitable, collection, attribute, text } from 'ember-cli-page-object';
import filter from 'consul-ui/tests/pages/components/catalog-filter';

export default create({
  visit: visitable('/:dc/services/:service'),
  nodes: collection('[data-test-node]', {
    name: attribute('data-test-node'),
  }),
  healthy: collection('[data-test-healthy] [data-test-node]', {
    name: attribute('data-test-node'),
    address: text('header strong'),
  }),
  unhealthy: collection('[data-test-unhealthy] [data-test-node]', {
    name: attribute('data-test-node'),
    address: text('header strong'),
  }),
  filter: filter,
});
