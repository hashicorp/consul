import { create, clickable, triggerable, is } from 'ember-cli-page-object';
import { visitable } from 'consul-ui/tests/lib/page-object/visitable';

export default create({
  // custom visitable
  visit: visitable(['/:dc/acls/:acl', '/:dc/acls/create']),
  // fillIn: fillable('input, textarea, [contenteditable]'),
  name: triggerable('keypress', '[name="name"]'),
  submit: clickable('[type=submit]'),
  submitIsEnabled: is(':not(:disabled)', '[type=submit]'),
});
