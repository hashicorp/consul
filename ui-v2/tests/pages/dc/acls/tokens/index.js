export default function(visitable, creatable, text, tokens, filter) {
  return {
    visit: visitable('/:dc/acls/tokens'),
    update: text('[data-test-notification-update]'),
    tokens: tokens(),
    filter: filter(),
    ...creatable(),
  };
}
