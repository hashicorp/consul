import { create, clickable } from 'ember-cli-page-object';
import { visitable } from 'consul-ui/tests/lib/page-object/visitable';

export default create({
  // custom visitable
  visit: visitable(['/:dc/kv/:kv/edit', '/:dc/kv/create'], str => str),
  // fillIn: fillable('input, textarea, [contenteditable]'),
  // name: triggerable('keypress', '[name="additional"]'),
  submit: clickable('[type=submit]'),
});
