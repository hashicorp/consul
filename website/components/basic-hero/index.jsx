import Button from '@hashicorp/react-button'

export default function BasicHero({
  heading,
  content,
  links,
  brand,
  backgroundImage,
}) {
  return (
    <div className={`g-basic-hero ${backgroundImage ? 'has-background' : ''}`}>
      <div className="g-grid-container">
        <h1 className="g-type-display-1">{heading}</h1>
        {content && <p className="g-type-body-large">{content}</p>}
        {links && links.length > 0 && (
          <div className="links">
            {links.map((link, stableIdx) => {
              let buttonVariant
              switch (stableIdx) {
                case 0:
                  buttonVariant = 'primary'
                  break
                case 1:
                  buttonVariant = 'secondary'
                  break
                case 2:
                  buttonVariant = 'tertiary'
                  break
                default:
                  break
              }
              const linkType = link.type || 'inbound'

              return (
                <Button
                  // eslint-disable-next-line react/no-array-index-key
                  key={stableIdx}
                  linkType={linkType}
                  theme={{
                    variant: buttonVariant,
                    brand,
                    background: 'light',
                  }}
                  title={link.text}
                  url={link.url}
                />
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
