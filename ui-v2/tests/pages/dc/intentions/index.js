export default function(visitable, creatable, intentions, filter) {
  return creatable({
    visit: visitable('/:dc/intentions'),
    intentions: intentions(),
    filter: filter('[data-test-intention-filter]'),
  });
}
