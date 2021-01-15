(function(doc, appName) {
  try {
    const $appMeta = doc.querySelector(`[name="${appName}/config/environment"]`);
    const consulConfig = JSON.parse(doc.querySelector(`[data-${appName}-config]`).textContent);
    const emberConfig = JSON.parse(decodeURIComponent($appMeta.getAttribute('content')));

    // rootURL is a special variable that requires settings before ember
    // boots via ember's HTML metadata tag, the variable is equivalent to
    // the -ui-content-path Consul flag (or `ui_config { content_path = ""}`)
    // There will potentially be one or two more 'pre-init' variables that we need.
    // Anything not 'pre-init' should use ui_config.
    // Sanity check the value to make sure its there and a string
    const rootURL = typeof consulConfig.ContentPath !== 'string' ? '' : consulConfig.ContentPath;
    if (rootURL.length > 0) {
      emberConfig.rootURL = rootURL;
    }
    $appMeta.setAttribute('content', encodeURIComponent(JSON.stringify(emberConfig)));
  } catch (e) {
    throw new Error(`Unable to parse ${appName} settings: ${e.message}`);
  }
})(document, 'consul-ui');
