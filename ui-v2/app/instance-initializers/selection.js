const isSelecting = function(win = window) {
  const selection = win.getSelection();
  let selecting = false;
  try {
    selecting = 'isCollapsed' in selection && !selection.isCollapsed;
  } catch (e) {
    // passthrough
  }
  return selecting;
};
export default {
  name: 'selection',
  initialize(container) {
    const dom = container.lookup('service:dom');
    const findAnchor = function(el) {
      return el.tagName === 'A' ? el : dom.closest('a', el);
    };
    const mousedown = function(e) {
      const $a = findAnchor(e.target);
      if ($a) {
        const href = $a.getAttribute('href');
        if (href) {
          $a.dataset.href = href;
          $a.removeAttribute('href');
        }
      }
    };
    const mouseup = function(e) {
      const $a = findAnchor(e.target);
      if ($a) {
        const href = $a.dataset.href;
        if (!isSelecting() && href) {
          $a.setAttribute('href', href);
        }
      }
    };

    document.body.addEventListener('mousedown', mousedown);
    document.body.addEventListener('mouseup', mouseup);

    container.reopen({
      willDestroy: function() {
        document.body.removeEventListener('mousedown', mousedown);
        document.body.removeEventListener('mouseup', mouseup);
        return this._super(...arguments);
      },
    });
  },
};
