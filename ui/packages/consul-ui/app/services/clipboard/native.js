import Service from '@ember/service';

const map = new WeakMap();
// we should only have one listener per event per element as we have a thin
// modifier layer over this.
// Event arrays are guaranteed to exist as the arrays are created at the same
// time as the functions themselves, if there are no arrays there are also no
// functions to be called.
const EVENTS = ['success', 'error'];
const addEventListener = function(eventName, cb) {
  if (EVENTS.includes(eventName)) {
    map.get(this)[eventName].push(cb);
  }
  return this;
};
const removeEventListener = function(eventName, cb) {
  if (EVENTS.includes(eventName)) {
    let listeners = map.get(this)[eventName];
    const pos = listeners.findIndex(item => item === cb);
    if (pos !== -1) {
      listeners.splice(pos, 1);
    }
  }
  return this;
};
//
export default class OsService extends Service {
  constructor(owner, clipboard = window.navigator.clipboard) {
    super(...arguments);
    this.clipboard = clipboard;
  }
  execute($el, options) {
    // make a pseudo-target that follows the ClipboardJS emitter until we want
    // to reverse the interface
    const target = {
      on: addEventListener,
      off: removeEventListener,
    };
    // we only want to support clicking/pressing enter
    const click = e => {
      const text = options.text();
      // ClipboardJS events also have action and trigger props
      // but we don't use them
      this.clipboard.writeText(text).then(
        () => map.get(target).success.forEach(cb => cb({ text: text })),
        e => map.get(target).error.forEach(cb => cb(e))
      );
    };
    // add all the events as empty arrays
    map.set(
      target,
      EVENTS.reduce((prev, item) => {
        prev[item] = [];
        return prev;
      }, {})
    );
    // listen plus remove using ClipboardJS interface
    $el.addEventListener('click', click);
    target.destroy = function() {
      $el.removeEventListener('click', click);
      map.delete(target);
    };
    return target;
  }
}
