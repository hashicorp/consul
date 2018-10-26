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
    };
    listeners.push(remove);
    return remove;
  };
  // TODO: Allow passing of a 'listener remove' in here
  // call it, find in the array and remove
  const remove = function() {
    listeners.forEach(item => item());
    listeners.splice(0, listeners.length);
    return listeners;
  };
  return {
    add: add,
    remove: remove,
  };
}
