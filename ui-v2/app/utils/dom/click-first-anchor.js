const clickEvent = function($el) {
  ['mousedown', 'mouseup', 'click']
    .map(function(type) {
      return new MouseEvent(type, {
        bubbles: true,
        cancelable: true,
        view: window,
      });
    })
    .forEach(function(event) {
      $el.dispatchEvent(event);
    });
};
export default function(closest, click = clickEvent) {
  // TODO: Decide whether we should use `e` for ease
  // or `target`/`el`
  // TODO: currently, using a string stopElement to tell the func
  // where to stop looking and currenlty default is 'tr' because
  // it's backwards compatible.
  // Long-term this func shouldn't default to 'tr'
  return function(e, stopElement = 'tr') {
    // click on row functionality
    // so if you click the actual row but not a link
    // find the first link and fire that instead
    const name = e.target.nodeName.toLowerCase();
    switch (name) {
      case 'input':
      case 'label':
      case 'a':
      case 'button':
        return;
    }
    const $a = closest(stopElement, e.target).querySelector('a');
    if ($a) {
      click($a);
    }
  };
}
