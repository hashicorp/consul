export default function (visitable, creatable, clickable, intentions, popoverSelect) {
  return {
    visit: visitable('/:dc/intentions'),
    intentionList: intentions(),
    sort: popoverSelect('[data-test-sort-control]'),
    ...creatable({}),
  };
}
