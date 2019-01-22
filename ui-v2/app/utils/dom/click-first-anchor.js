const clickEvent = function() {
  return new MouseEvent('click', {
    bubbles: true,
    cancelable: true,
    view: window,
  });
};
export default function(closest, click = clickEvent) {
  // TODO: Decide whether we should use `e` for ease
  // or `target`/`el`
  return function(e) {
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
    // TODO: why should this be restricted to a tr
    // closest should probably be relaced with a finder function
    const $a = closest('tr', e.target).querySelector('a');
    if ($a) {
      $a.dispatchEvent(click());
    }
  };
}
