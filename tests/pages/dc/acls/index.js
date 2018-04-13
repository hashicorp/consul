import { create, visitable, collection } from 'ember-cli-page-object';

export default create({
  visit: visitable('/:dc/acls'),
  acls: collection('[data-test-acl]'),
});
