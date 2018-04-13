import { create, visitable, collection } from 'ember-cli-page-object';

export default create({
  visit: visitable('/:dc/nodes/'),
  nodes: collection('[data-test-node]'),
});
