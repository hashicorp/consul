export default function (visitable, submitable, isPresent) {
  return submitable({
    visit: visitable('/setting'),
    blockingQueries: isPresent('[data-test-blocking-queries]'),
  });
}
