import ReactTabs from '@hashicorp/react-tabs'

export function Tabs({ children }) {
  if (!Array.isArray(children))
    throw new Error('Multiple <Tab> elements required')

  return (
    <ReactTabs
      items={children.map((Block) => ({
        heading: Block.props.heading,
        // eslint-disable-next-line react/display-name
        tabChildren: () => Block,
      }))}
    />
  )
}

export function Tab({ children }) {
  return <>{children}</>
}
