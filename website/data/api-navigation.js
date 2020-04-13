// The root folder for this documentation category is `pages/guides`
//
// - A string refers to the name of a file
// - A "category" value refers to the name of a directory
// - All directories must have an "index.mdx" file to serve as
//   the landing page for the category, or a "name" property to
//   serve as the category title in the sidebar

export default [
  'index',
  {
    category: 'features',
    name: 'API Features',
    content: ['consistency', 'blocking', 'filtering', 'caching'],
  },
  '--------',
  {
    category: 'acl',
    content: [
      'tokens',
      'legacy',
      'policies',
      'roles',
      'auth-methods',
      'binding-rules',
    ],
  },
  {
    category: 'agent',
    content: ['check', 'service', 'connect'],
  },
  'catalog',
  'config',
  { category: 'connect', content: ['ca', 'intentions'] },
  'coordinate',
  'discovery-chain',
  'event',
  'health',
  'kv',
  {
    category: 'operator',
    content: ['area', 'autopilot', 'keyring', 'license', 'raft', 'segment'],
  },
  'namespaces',
  'query',
  'session',
  'snapshot',
  'status',
  'txn',
  '-------',
  'libraries-and-sdks',
]
