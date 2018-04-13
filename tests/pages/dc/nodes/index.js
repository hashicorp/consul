import { create, visitable, collection, attribute, clickable } from 'ember-cli-page-object';
import filter from 'consul-ui/tests/pages/components/catalog-filter';

export default create({
  visit: visitable('/:dc/nodes'),
  nodes: collection('[data-test-node]', {
    name: attribute('data-test-node'),
    node: clickable('header a'),
  }),
  filter: filter,
});
