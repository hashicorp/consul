import { create, visitable, collection, attribute, clickable } from 'ember-cli-page-object';

import filter from 'consul-ui/tests/pages/components/acl-filter';
export default create({
  visit: visitable('/:dc/acls'),
  acls: collection('[data-test-tabular-row]', {
    name: attribute('data-test-acl', '[data-test-acl]'),
    acl: clickable('a'),
    actions: clickable('label'),
    delete: clickable('[data-test-delete]'),
    confirmDelete: clickable('button.type-delete'),
  }),
  filter: filter,
});
