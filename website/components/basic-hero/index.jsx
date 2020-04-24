import Button from '@hashicorp/react-button'

export default function BasicHero({
  heading,
  content,
  links,
  brand,
  backgroundImage,
}) {
  console.log('background?', backgroundImage)
  return (
    <div className={`g-basic-hero ${backgroundImage ? 'has-background' : ''}`}>
      <div className="g-grid-container">
        <h1 className="g-type-display-1">{heading}</h1>
        {content && <p className="g-type-body-large">{content}</p>}
        {links && links.length > 0 && (
          <div className="links">
            {links.map((link, stableIdx) => {
              const buttonVariant = stableIdx === 0 ? 'primary' : 'secondary'
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
