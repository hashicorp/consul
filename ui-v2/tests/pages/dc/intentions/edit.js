export default function(visitable, submitable, deletable) {
  return submitable(
    deletable({
      visit: visitable(['/:dc/intentions/:intention', '/:dc/intentions/create']),
    })
  );
}
