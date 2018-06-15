import { create, clickable } from 'ember-cli-page-object';
import { visitable } from 'consul-ui/tests/lib/page-object/visitable';

export default create({
  // custom visitable
  visit: visitable(['/:dc/intentions/:intention', '/:dc/intentions/create']),
  submit: clickable('[type=submit]'),
});
