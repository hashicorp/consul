export default function(visitable, creatable, clickable, intentions, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/intentions'),
    intentions: intentions(),
    sort: popoverSelect(),
    create: clickable('[data-test-create]'),
  });
}
