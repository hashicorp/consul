// This script removes any `layout` keys in mdx files in a given directory,
// recursively. In this project, we use a default layout for all topic content,
// so the layout key is not necessary unless a topic needs to render into a
// unique layout

const glob = require('glob')
const path = require('path')
const fs = require('fs')
const matter = require('gray-matter')

glob.sync(path.join(__dirname, '../pages/**/*.mdx')).map((fullPath) => {
  let { content, data } = matter.read(fullPath)
  content = content.replace(
    /<%=\s*partial[(\s]["'](.*)["'][)\s]\s*%>/gm,
    (_, partialPath) => `@include '${partialPath}.mdx'`
  )
  fs.writeFileSync(fullPath, matter.stringify(content, data))
})
