import LinkComponent from '@ember/routing/link-component';

export class HrefTo {
  constructor(container, target) {
    this.applicationInstance = container;
    this.target = target;
    const hrefAttr = this.target.attributes.href;
    this.url = hrefAttr && hrefAttr.value;
  }

  handle(e) {
    if (this.shouldHandle(e)) {
      e.preventDefault();
      this.applicationInstance.lookup('router:main').location.transitionTo(this.url);
    }
  }

  shouldHandle(e) {
    return (
      this.isUnmodifiedLeftClick(e) &&
      !this.isIgnored(this.target) &&
      !this.isExternal(this.target) &&
      !this.hasActionHelper(this.target) &&
      !this.hasDownload(this.target) &&
      !this.isLinkComponent(this.target)
    );
    // && this.recognizeUrl(this.url);
  }

  isUnmodifiedLeftClick(e) {
    return (e.which === undefined || e.which === 1) && !e.ctrlKey && !e.metaKey;
  }

  isExternal($el) {
    return $el.getAttribute('target') === '_blank';
  }

  isIgnored($el) {
    return $el.dataset.nativeHref;
  }

  hasActionHelper($el) {
    return $el.dataset.emberAction;
  }

  hasDownload($el) {
    return $el.hasAttribute('download');
  }

  isLinkComponent($el) {
    let isLinkComponent = false;
    const id = $el.id;
    if (id) {
      const componentInstance = this.applicationInstance.lookup('-view-registry:main')[id];
      isLinkComponent = componentInstance && componentInstance instanceof LinkComponent;
    }
    return isLinkComponent;
  }

  recognizeUrl(url) {
    let didRecognize = false;

    if (url) {
      const router = this._getRouter();
      const rootUrl = this._getRootUrl();
      const isInternal = url.indexOf(rootUrl) === 0;
      const urlWithoutRoot = this.getUrlWithoutRoot();
      const routerMicrolib = router._router._routerMicrolib || router._router.router;

      didRecognize = isInternal && routerMicrolib.recognizer.recognize(urlWithoutRoot);
    }

    return didRecognize;
  }

  getUrlWithoutRoot() {
    const location = this.applicationInstance.lookup('router:main').location;
    let url = location.getURL.apply(
      {
        getHash: () => '',
        location: {
          pathname: this.url,
        },
        baseURL: location.baseURL,
        rootURL: location.rootURL,
        env: location.env,
      },
      []
    );
    const pos = url.indexOf('?');
    if (pos !== -1) {
      url = url.substr(0, pos - 1);
    }
    return url;
  }

  _getRouter() {
    return this.applicationInstance.lookup('service:router');
  }

  _getRootUrl() {
    let router = this._getRouter();
    let rootURL = router.get('rootURL');

    if (rootURL.charAt(rootURL.length - 1) !== '/') {
      rootURL = rootURL + '/';
    }

    return rootURL;
  }
}
function closestLink(el) {
  if (el.closest) {
    return el.closest('a');
  } else {
    el = el.parentElement;
    while (el && el.tagName !== 'A') {
      el = el.parentElement;
    }
    return el;
  }
}
export default {
  name: 'href-to',
  initialize(container) {
    // we only want this to run in the browser, not in fastboot
    if (typeof FastBoot === 'undefined') {
      const dom = container.lookup('service:dom');
      const doc = dom.document();

      const listener = e => {
        const link = e.target.tagName === 'A' ? e.target : closestLink(e.target);
        if (link) {
          const hrefTo = new HrefTo(container, link);
          hrefTo.handle(e);
        }
      };

      doc.body.addEventListener('click', listener);
      container.reopen({
        willDestroy() {
          doc.body.removeEventListener('click', listener);
          return this._super(...arguments);
        },
      });
    }
  },
};
