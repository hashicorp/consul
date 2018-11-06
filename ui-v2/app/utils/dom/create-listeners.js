export default function(listeners = []) {
  const add = function(target, event, handler) {
    let addEventListener = 'addEventListener';
    let removeEventListener = 'removeEventListener';
    if (typeof target[addEventListener] === 'undefined') {
      addEventListener = 'on';
      removeEventListener = 'off';
    }
    target[addEventListener](event, handler);
    const remove = function() {
      target[removeEventListener](event, handler);
      return handler;
    };
    listeners.push(remove);
    return remove;
  };
  // TODO: Allow passing of a 'listener remove' in here
  // call it, find in the array and remove
  // Post-thoughts, pretty sure this is covered now by returning the remove
  // function above, use-case for wanting to use this method to remove individual
  // listeners is probably pretty limited, this method itself could be easily implemented
  // from the outside also, but I suppose its handy to keep here
  const remove = function() {
    const handlers = listeners.map(item => item());
    listeners.splice(0, listeners.length);
    return handlers;
  };
  return {
    add: add,
    remove: remove,
  };
}
