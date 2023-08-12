/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

(function(doc, appName) {
  const fs = new Map(
    Object.entries(JSON.parse(doc.querySelector(`[data-${appName}-fs]`).textContent))
  );
  const appendScript = function(src) {
    var $script = doc.createElement('script');
    $script.src = src;
    doc.body.appendChild($script);
  };

  // polyfills
  if (!('TextDecoder' in window)) {
    appendScript(fs.get(`${['text-encoding', 'encoding-indexes'].join('/')}.js`));
    appendScript(fs.get(`${['text-encoding', 'encoding'].join('/')}.js`));
  }
  if (!(window.CSS && window.CSS.escape)) {
    appendScript(fs.get(`${['css.escape', 'css.escape'].join('/')}.js`));
  }

  try {
    const $appMeta = doc.querySelector(`[name="${appName}/config/environment"]`);
    // pick out the operatorConfig from our application/json script tag
    const operatorConfig = JSON.parse(doc.querySelector(`[data-${appName}-config]`).textContent);
    // pick out the ember config from its meta tag
    const emberConfig = JSON.parse(decodeURIComponent($appMeta.getAttribute('content')));

    // rootURL is a special variable that requires settings before ember
    // boots via ember's HTML metadata tag, the variable is equivalent to
    // the -ui-content-path Consul flag (or `ui_config { content_path = ""}`)
    // There will potentially be one or two more 'pre-init' variables that we need.
    // Anything not 'pre-init' should use ui_config.
    // Check the value to make sure its there and a string
    const rootURL =
      typeof operatorConfig.ContentPath !== 'string' ? '' : operatorConfig.ContentPath;
    if (rootURL.length > 0) {
      emberConfig.rootURL = rootURL;
    }
    $appMeta.setAttribute('content', encodeURIComponent(JSON.stringify(emberConfig)));
  } catch (e) {
    throw new Error(`Unable to parse ${appName} settings: ${e.message}`);
  }
})(document, 'consul-ui');
