import { env } from 'consul-ui/env';

const SECONDARY_BUTTON = 2;
const isSelecting = function(win = window) {
  const selection = win.getSelection();
  let selecting = false;
  try {
    selecting =
      'isCollapsed' in selection && !selection.isCollapsed && selection.toString().length > 1;
  } catch (e) {
    // passthrough
  }
  return selecting;
};
export default {
  name: 'selection',
  initialize(container) {
    if (env('CONSUL_UI_DISABLE_ANCHOR_SELECTION')) {
      return;
    }
    const dom = container.lookup('service:dom');
    const doc = dom.document();
    const $html = doc.getElementsByTagName('html')[0];
    const findAnchor = function(el) {
      return el.tagName === 'A' ? el : dom.closest('a', el);
    };
    const mousedown = function(e) {
      if ($html.classList.contains('is-debug')) {
        return;
      }
      const $a = findAnchor(e.target);
      if ($a) {
        if (typeof e.button !== 'undefined' && e.button === SECONDARY_BUTTON) {
          const dataHref = $a.dataset.href;
          if (dataHref) {
            $a.setAttribute('href', dataHref);
          }
          return;
        }
        const href = $a.getAttribute('href');
        if (href) {
          $a.dataset.href = href;
          $a.removeAttribute('href');
        }
      }
    };
    const mouseup = function(e) {
      if ($html.classList.contains('is-debug')) {
        return;
      }
      const $a = findAnchor(e.target);
      if ($a) {
        const href = $a.dataset.href;
        if (!isSelecting() && href) {
          $a.setAttribute('href', href);
        }
      }
    };

    doc.body.addEventListener('mousedown', mousedown);
    doc.body.addEventListener('mouseup', mouseup);

    container.reopen({
      willDestroy: function() {
        doc.body.removeEventListener('mousedown', mousedown);
        doc.body.removeEventListener('mouseup', mouseup);
        return this._super(...arguments);
      },
    });
  },
};
