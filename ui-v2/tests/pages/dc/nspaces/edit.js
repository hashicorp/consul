export default function(visitable, submitable, deletable, cancelable) {
  return {
    visit: visitable(['/:dc/namespaces/:namespace', '/:dc/namespaces/create']),
    ...submitable({}, 'form > div'),
    ...cancelable({}, 'form > div'),
    ...deletable({}, 'form > div'),
  };
}
