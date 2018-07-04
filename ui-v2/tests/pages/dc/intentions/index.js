import { create, visitable, collection, attribute, clickable } from 'ember-cli-page-object';

import filter from 'consul-ui/tests/pages/components/intention-filter';
export default create({
  visit: visitable('/:dc/intentions'),
  intentions: collection('[data-test-tabular-row]', {
    source: attribute('data-test-intention-source', '[data-test-intention-source]'),
    destination: attribute('data-test-intention-destination', '[data-test-intention-destination]'),
    action: attribute('data-test-intention-action', '[data-test-intention-action]'),
    intention: clickable('a'),
    actions: clickable('label'),
    delete: clickable('[data-test-delete]'),
    confirmDelete: clickable('button.type-delete'),
  }),
  filter: filter,
});
