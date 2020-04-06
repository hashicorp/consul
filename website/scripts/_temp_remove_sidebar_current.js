// This script removes the "sidebar_current" key from frontmatter, as it is
// no longer needed.

const glob = require('glob')
const path = require('path')
const fs = require('fs')
const matter = require('gray-matter')

glob.sync(path.join(__dirname, '../pages/**/*.mdx')).map((fullPath) => {
  let { content, data } = matter.read(fullPath)
  delete data.sidebar_current
  fs.writeFileSync(fullPath, matter.stringify(content, data))
})
