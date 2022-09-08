export const selectors = {
  $: '.consul-peer-list',
  collection: {
    $: '[data-test-list-row]',
    peer: {
      $: 'li'
    },
  }
};
export default (collection, isPresent) => () => {
  return collection(`${selectors.$} ${selectors.collection.$}`, {
    peer: isPresent(selectors.collection.peer.$),
  });
};
