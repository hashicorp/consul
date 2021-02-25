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
      root: path.resolve(__dirname, 'app/components'),
      pattern: '**/README.mdx',
      urlSchema: 'auto',
      urlPrefix: 'docs/components',
    }
  ],
};
