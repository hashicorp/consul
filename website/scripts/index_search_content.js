require('dotenv').config()

const algoliasearch = require('algoliasearch')
const glob = require('glob')
const matter = require('gray-matter')
const path = require('path')
const remark = require('remark')
const visit = require('unist-util-visit')

// In addition to the content of the page,
// define additional front matter attributes that will be search-indexable
const SEARCH_DIMENSIONS = ['page_title', 'description']

main()

async function main() {
  const pagesFolder = path.join(__dirname, '../pages')

  // Grab all search-indexable content and format for Algolia
  const searchObjects = await Promise.all(
    glob.sync(path.join(pagesFolder, '**/*.mdx')).map(async (fullPath) => {
      const { content, data } = matter.read(fullPath)

      const searchableDimensions = SEARCH_DIMENSIONS.reduce(
        (acc, dimension) => {
          return { ...acc, [dimension]: data[dimension] }
        },
        {}
      )

      const headings = await collectHeadings(content)

      // Get path relative to `pages`
      const __resourcePath = fullPath.replace(`${pagesFolder}/`, '')

      // Use clean URL for Algolia id
      const objectID = __resourcePath.replace('.mdx', '')

      return {
        ...searchableDimensions,
        headings,
        objectID,
      }
    })
  )

  try {
    await indexSearchContent(searchObjects)
  } catch (e) {
    console.error(e)
    process.exit(1)
  }
}

async function indexSearchContent(objects) {
  const {
    NEXT_PUBLIC_ALGOLIA_APP_ID: appId,
    NEXT_PUBLIC_ALGOLIA_INDEX: index,
    ALGOLIA_API_KEY: apiKey,
  } = process.env

  if (!apiKey || !appId || !index) {
    throw new Error(
      `[*** Algolia Search Indexing Error ***] Received: ALGOLIA_API_KEY=${apiKey} ALGOLIA_APP_ID=${appId} ALGOLIA_INDEX=${index} \n Please ensure all Algolia Search-related environment vars are set in CI settings.`
    )
  }

  console.log(`updating ${objects.length} indices...`)

  try {
    const searchClient = algoliasearch(appId, apiKey)
    const searchIndex = searchClient.initIndex(index)

    const { objectIDs } = await searchIndex.partialUpdateObjects(objects, {
      createIfNotExists: true,
    })

    let staleIds = []

    await searchIndex.browseObjects({
      query: '',
      batch: (batch) => {
        staleIds = staleIds.concat(
          batch
            .filter(({ objectID }) => !objectIDs.includes(objectID))
            .map(({ objectID }) => objectID)
        )
      },
    })

    if (staleIds.length > 0) {
      console.log(`deleting ${staleIds.length} stale indices:`)
      console.log(staleIds)

      await searchIndex.deleteObjects(staleIds)
    }

    console.log('done')
    process.exit(0)
  } catch (error) {
    throw new Error(error)
  }
}

async function collectHeadings(mdxContent) {
  const headings = []

  const headingMapper = () => (tree) => {
    visit(tree, 'heading', (node) => {
      const title = node.children.reduce((m, n) => {
        if (n.value) m += n.value
        return m
      }, '')
      // Only include level 1 or level 2 headings
      if (node.depth < 3) {
        headings.push(title)
      }
    })
  }

  return remark()
    .use(headingMapper)
    .process(mdxContent)
    .then(() => headings)
}
