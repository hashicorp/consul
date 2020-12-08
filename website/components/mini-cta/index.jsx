import Button from '@hashicorp/react-button'

export default function MiniCTA({ title, description, link }) {
  return (
    <div className="g-mini-cta">
      <div className="g-grid-container">
        <hr />
        <h5 className="g-type-display-4">{title}</h5>
        {description && <p className="g-type-body">{description}</p>}
        <Button
          title={link.text}
          url={link.url}
          theme={{
            variant: 'tertiary-neutral',
            brand: 'neutral',
            background: 'light'
          }}
          linkType={link.type}
        />
      </div>
    </div>
  )
}
