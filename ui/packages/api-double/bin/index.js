#!/usr/bin/env node
//
const $ = process.env;
const fs = require('fs');
const path = require('path');
const promisify = require('util').promisify;
const read = promisify(fs.readFile);
//
const bodyParser = require('body-parser');
const cookieParser = require('cookie-parser');
//
const __ = process.argv.reduce(
  function(prev, flag, i, arr) {
    const val = () => arr[i + 1];
    switch(true) {
      case flag === '--port':
        prev.port = val();
        break;
      case flag === '--dir':
        prev.dir = val();
        break;
      case flag === '--seed':
        prev.seed = val();
        break;
    }
    return prev;
  },
  {
    port: $.HC_API_DOUBLE_PORT || 3000,
    dir: $.HC_API_DOUBLE_DIR || './',
    seed: $.HC_API_DOUBLE_SEED,
  }
);
const dir = path.resolve(__.dir);
const controller = require('../index.js')(__.seed, dir, read, $, path.resolve);
[
  require('../lib/headers')(),
  require('cookie-parser')(),
  require('body-parser').text({type: '*/*'}),
  controller().serve
].reduce(
  function(app, item) {
    return app.use(item);
  },
  require('express')()
).listen(
  __.port,
  function() {
    console.log(`Listening on port ${__.port}, using ${dir}`);
  }
);



