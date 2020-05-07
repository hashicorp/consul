'use strict';
const fs = require('fs');
const promisify = require('util').promisify;
const read = promisify(fs.readFile);

module.exports = function(app, options) {
  // During development the proxy server has no way of
  // knowing the content/mime type of our `oidc/callback` file
  // as it has no extension.
  // This shims the default server to set the correct headers
  // just for this file

  const file = `/oidc/callback`;
  const rootURL = options.rootURL;
  const url = `${rootURL.substr(0, rootURL.length - 1)}${file}`;
  app.use(function(req, resp, next) {
    if (req.url.split('?')[0] === url) {
      return read(`${process.cwd()}/public${file}`).then(function(buffer) {
        resp.header('Content-Type', 'text/html');
        resp.write(buffer.toString());
        resp.end();
      });
    }
    next();
  });
};
