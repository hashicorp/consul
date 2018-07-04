export default function(visitable, deletable, clickable, attribute, collection) {
  return {
    visit: visitable('/:dc/kv'),
    kvs: collection(
      '[data-test-tabular-row]',
      deletable({
        name: attribute('data-test-kv', '[data-test-kv]'),
        kv: clickable('a'),
        actions: clickable('label'),
      })
    ),
  };
}
