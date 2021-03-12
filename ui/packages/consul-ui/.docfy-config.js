const path = require('path');
const autolinkHeadings = require('remark-autolink-headings');
const refractor = require('refractor');
const prism = require('@mapbox/rehype-prism');

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
      root: path.resolve(__dirname, 'app/modifiers'),
      pattern: '**/*.mdx',
      urlSchema: 'auto',
      urlPrefix: 'docs/modifiers',
    },
    {
      root: path.resolve(__dirname, 'app/routing'),
      pattern: '**/*.mdx',
      urlSchema: 'auto',
      urlPrefix: 'docs/routing',
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
    }
  ],
  labels: {
    "consul": "Consul Components"
  }
};
