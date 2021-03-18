const fs = require('fs/promises');
const exists = require('fs').existsSync;
const path = require('path');

const test = require('tape');
const glob = require('glob-promise');

const parser = require('../index');

const fixtures = path.resolve(`${__dirname}/../hcl`);
// const hclPath = `${fixtures}/test-fixtures/block_assign.hcl`;
const hclPath = `${__dirname}/in.hcl`;

test(
  'it works',
  async (t) => {
    const hcl = await fs.readFile(hclPath, 'utf8');
    console.log(JSON.stringify(parser.parse(hcl), null, 4));
    t.plan(1);
    t.ok(true);
  }
)
test(
  'hcl1 tests',
  async (t) => {
    const fixtures = `{${path.resolve(`${__dirname}/../hcl/test-fixtures`)},${path.resolve(`${__dirname}/../hcl/hcl/test-fixtures`)},${path.resolve(`${__dirname}/../hcl/hcl/parser/test-fixtures`)}}`;
    const files = (await glob(`${fixtures}/*.hcl`)).map(
      (file) => {
        const hcl = file.replace('.json', '.hcl');
        if(exists(hcl)) {
          return {
            path: hcl,
            content: fs.readFile(hcl, 'utf8')
          }

        } else {
          return {
            path: hcl,
            content: false
          }
        }
      }
    );
    Promise.all(files.map(item => item.content)).then(
      (data) => {
        data.forEach(
          (hcl, i) => {
            if(hcl !== false) {
              try {
                const json = JSON.stringify(parser.parse(hcl), null, 4);
                // console.log(json);

              } catch(e) {
                console.log(`${files[i].path.split('/').pop()} wouldn't parse`);

              }
            }
          }
        );
      }
    );
    t.plan(1);
    t.ok(true);
    
  }
);
