import { create, visitable, collection, attribute, clickable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/:dc/kv'),
  kvs: collection('[data-test-tabular-row]', {
    name: attribute('data-test-kv', '[data-test-kv]'),
    kv: clickable('a'),
    actions: clickable('label'),
    delete: clickable('[data-test-delete]'),
    confirmDelete: clickable('button.type-delete'),
  }),
});
