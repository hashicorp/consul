export default function(visitable, submitable, deletable) {
  return submitable(
    deletable({
      visit: visitable(['/:dc/kv/:kv/edit', '/:dc/kv/create'], str => str),
    })
  );
}
