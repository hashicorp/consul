import { text } from 'ember-cli-page-object';

export default function(visitable, isPresent) {
  return {
    visit: visitable('/:dc/routing-config/:name'),
    source: text('[data-test-consul-source]'),
  };
}
