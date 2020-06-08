import Button from '@hashicorp/react-button'
import InlineSvg from '@hashicorp/react-inline-svg'

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
            {links[2] && (
              <div className="third-link">
                <a href={links[2].url}>
                  <span className="g-type-buttons-and-standalone-links">
                    {links[2].text}
                  </span>
                  <span className="icon">
                    <InlineSvg
                      src={
                        '<svg role="img" width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M20 12L14 18M4 12H20H4ZM20 12L14 6L20 12Z" stroke="#76767D" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>'
                      }
                    />
                  </span>
                </a>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
