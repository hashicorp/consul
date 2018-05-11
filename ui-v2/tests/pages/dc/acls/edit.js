import { create, visitable, clickable, triggerable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/:dc/acls/:acl'),
  // fillIn: fillable('input, textarea, [contenteditable]'),
  name: triggerable('keypress', '[name="name"]'),
  submit: clickable('[type=submit]'),
});
