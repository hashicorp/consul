/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/*eslint node/no-extraneous-require: "off"*/
'use strict';
const fs = require('fs');
const promisify = require('util').promisify;
const read = promisify(fs.readFile);
const express = require('express');

module.exports = function (app, options) {
  // During development the proxy server has no way of
  // knowing the content/mime type of our `oidc/callback` file
  // as it has no extension.
  // This shims the default server to set the correct headers
  // just for this file

  const file = `/oidc/callback`;
  const rootURL = options.rootURL;
  const url = `${rootURL.substr(0, rootURL.length - 1)}${file}`;
  app.use(function (req, resp, next) {
    if (req.url.split('?')[0] === url) {
      return read(`${process.cwd()}/public${file}`).then(function (buffer) {
        resp.header('Content-Type', 'text/html');
        resp.write(buffer.toString());
        resp.end();
      });
    }
    next();
  });

  // sets the base CSP policy for the UI
  app.use(function (request, response, next) {
    response.set({
      'Content-Security-Policy': `default-src 'self' 'unsafe-inline' ws: localhost:${options.liveReloadPort} http: localhost:${options.liveReloadPort}; img-src 'self' data: ; style-src 'self' 'unsafe-inline'`,
    });
    next();
  });
  // Serve the coverage folder for easy viewing during development
  app.use('/coverage', express.static('coverage'));
};
