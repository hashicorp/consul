// quick hack around not being able to pass an empty
// string as a wildcard route
// TODO: this is a breaking change, fix this
export default function(key, root) {
  return key === root ? '/' : key; // consider null check and return root, although this will probably go
}
