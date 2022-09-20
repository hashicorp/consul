export const selectors = {
  $: '.consul-peer-list',
  collection: {
    $: '[data-test-list-row]',
    peer: {
      $: 'li',
      name: {
        $: '[data-test-peer]'
      }
    },
  }
};
export default (collection, isPresent, attribute) => () => {
  return collection(`${selectors.$} ${selectors.collection.$}`, {
    peer: isPresent(selectors.collection.peer.$),
    name: attribute('data-test-peer', selectors.collection.peer.name.$),
  });
};
