import { create, visitable, clickable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/settings'),
  submit: clickable('[type=submit]'),
});
