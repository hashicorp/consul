import { create, visitable, fillable, clickable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/:dc/kv/:kv'),
  fillIn: fillable('input, textarea, [contenteditable]'),
  submit: clickable('[type=submit]'),
});
