const test = require('tape');
const { readFileSync } = require("fs");
const path = require("path");

const babel = require("@babel/core");

const plugin = require("../index");

const transform = function(code, vars) {
  const str = babel.transform(code, {
    plugins: [plugin],
  }).code;
  return new Function(`return ${str}`)();
}

test(
  'it transpiles HCL without interpolated variables',
  (t) => {
    t.plan(1);
    const expected = {
      "route": {
        "services": {
          "something":"here"
        }
      }
    };
    const actual = transform('hcl`route "services" { something = "here"}`');
    t.deepEquals(actual, expected);
  }
)
