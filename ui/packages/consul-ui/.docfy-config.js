const path = require('path');

const autolinkHeadings = require('remark-autolink-headings');
const refractor = require('refractor');
const gherkin = require('refractor/lang/gherkin');
const prism = require('@mapbox/rehype-prism');

const fs = require('fs');
const read = fs.readFileSync;
const exists = fs.existsSync;
const chalk = require('chalk'); // comes with ember

// allow extra docfy config
let user = {sources: [], labels: {}};
const $CONSUL_DOCFY_CONFIG = process.env.CONSUL_DOCFY_CONFIG || '';
if($CONSUL_DOCFY_CONFIG.length > 0) {
  try {
      if(exists($CONSUL_DOCFY_CONFIG)) {
        user = JSON.parse(read($CONSUL_DOCFY_CONFIG));
      } else {
        throw new Error(`Unable to locate ${$CONSUL_DOCFY_CONFIG}`);
      }
  } catch(e) {
    console.error(chalk.yellow(`Docfy: ${e.message}`));
  }
}

refractor.register(gherkin);
refractor.alias('handlebars', 'hbs');
refractor.alias('shell', 'sh');


module.exports = {
  remarkHbsOptions: {
    escapeCurliesCode: false
  },
  remarkPlugins: [
    autolinkHeadings,
    {
      behavior: 'wrap'
    }
  ],
  rehypePlugins: [
    prism
  ],
  sources: [
    {
      root: path.resolve(__dirname, 'docs'),
      pattern: '**/*.mdx',
      urlSchema: 'auto',
      urlPrefix: 'docs',
    },
    {
      root: path.resolve(__dirname, 'app/styles'),
      pattern: '**/*.mdx',
      urlSchema: 'auto',
      urlPrefix: 'docs/styles',
    },
    {
      root: path.resolve(__dirname, 'app/modifiers'),
      pattern: '**/*.mdx',
      urlSchema: 'auto',
      urlPrefix: 'docs/modifiers',
    },
    {
      root: path.resolve(__dirname, 'app/helpers'),
      pattern: '**/*.mdx',
      urlSchema: 'auto',
      urlPrefix: 'docs/helpers',
    },
    {
      root: path.resolve(__dirname, 'app/services'),
      pattern: '**/*.mdx',
      urlSchema: 'auto',
      urlPrefix: 'docs/services',
    },
    {
      root: path.resolve(__dirname, 'app/components'),
      pattern: '**(!consul)/README.mdx',
      urlSchema: 'auto',
      urlPrefix: 'docs/components',
    },
    {
      root: path.resolve(__dirname, 'app/components/consul'),
      pattern: '**/README.mdx',
      urlSchema: 'auto',
      urlPrefix: 'docs/consul',
    },
    {
      root: `${path.dirname(require.resolve('consul-partitions/package.json'))}/app/components`,
      pattern: '**/README.mdx',
      urlSchema: 'auto',
      urlPrefix: 'docs/consul-partitions',
    },
    {
      root: `${path.dirname(require.resolve('consul-nspaces/package.json'))}/app/components`,
      pattern: '**/README.mdx',
      urlSchema: 'auto',
      urlPrefix: 'docs/consul-nspaces',
    }
  ].concat(user.sources),
  labels: {
    "consul": "Consul Components",
    ...user.labels
  }
};
