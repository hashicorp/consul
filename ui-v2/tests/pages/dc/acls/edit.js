import { create, visitable, fillable, clickable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/:dc/acls/:acl'),
  fillIn: fillable('input, textarea, [contenteditable]'),
  submit: clickable('[type=submit]'),
});
