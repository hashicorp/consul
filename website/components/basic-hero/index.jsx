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
          <>
            <div className="links">
              {links.slice(0, 2).map((link, stableIdx) => {
                const buttonVariant = stableIdx === 0 ? 'primary' : 'secondary'
                return (
                  <Button
                    // eslint-disable-next-line react/no-array-index-key
                    key={stableIdx}
                    linkType={link.type}
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
            {links[2] && (
              <div className="third-link">
                <Button
                  // eslint-disable-next-line react/no-array-index-key
                  linkType={links[2].type}
                  theme={{
                    variant: 'tertiary-neutral',
                    brand,
                    background: 'light',
                  }}
                  title={links[2].text}
                  url={links[2].url}
                />
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
