/* eslint no-console: "off" */
/* eslint-env node */
'use strict';
const babel = require('@babel/core');
const read = require('fs').readFileSync;
const path = require('path');
const vm = require('vm');
const color = require('chalk');

const out = function (prefix, step, desc) {
  if (!Array.isArray(step)) {
    step = [step];
  }
  step.forEach(function (item) {
    const str =
      prefix +
      item.replace('\n', ' | ').replace(/\$\w+/g, function (match) {
        return color.cyan(match);
      });
    console.log(color.green(str));
  });
};
const library = {
  given: function (step, cb, desc) {
    out('Given ', step, desc);
    return this;
  },
  desc: function (desc) {
    console.log(color.yellow(`- ${desc.trim()}`));
  },
  section: function () {
    console.log(color.yellow(`##`));
  },
  then: function (step, cb, desc) {
    out('Then ', step, desc);
    return this;
  },
  when: function (step, cb, desc) {
    out('When ', step, desc);
    return this;
  },
};
const root = process.cwd();
const exec = function (filename) {
  const js = read(filename);
  const code = babel.transform(js.toString(), {
    filename: filename,
    presets: ['@babel/preset-env'],
  }).code;
  const exports = {};
  vm.runInNewContext(
    code,
    {
      exports: exports,
      require: function (str) {
        return exec(path.resolve(`${root}/tests`, `${str}.js`)).default;
      },
    },
    {
      filename: filename,
    }
  );
  return exports;
};

module.exports = function (filename) {
  const assert = () => {};
  exec(filename).default({ assert, library });
};
