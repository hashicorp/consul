import { create, visitable, collection } from 'ember-cli-page-object';

export default create({
  visit: visitable('/:dc/kv'),
  kvs: collection('[data-test-kv]'),
});
