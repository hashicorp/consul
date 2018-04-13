import { create, visitable, collection, attribute } from 'ember-cli-page-object';

import filter from 'consul-ui/tests/pages/components/acl-filter';
export default create({
  visit: visitable('/:dc/acls'),
  acls: collection('[data-test-acl]', {
    name: attribute('data-test-acl'),
  }),
  filter: filter,
});
