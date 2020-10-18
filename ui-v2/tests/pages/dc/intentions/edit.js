export default function(visitable, submitable, deletable, cancelable) {
  return submitable(
    cancelable(
      deletable({
        visit: visitable(['/:dc/intentions/:intention', '/:dc/intentions/create']),
      })
    ),
    'main'
  );
}
