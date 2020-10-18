export default function(visitable, creatable, clickable, intentions, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/intentions'),
    intentions: intentions(),
    sort: popoverSelect('[data-test-sort-control]'),
    create: clickable('[data-test-create]'),
  });
}
