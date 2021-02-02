import Image from '@hashicorp/react-image'
import Button from '@hashicorp/react-button'
import InlineSvg from '@hashicorp/react-inline-svg'
import ArrowIcon from './img/arrow.svg?include'

export default function EnterpriseComparison({
  title,
  itemOne,
  itemTwo,
  brand,
}) {
  return (
    <div className="g-enterprise-comparison">
      <div className="g-grid-container">
        <h2 className="g-type-display-2">{title}</h2>

        <div className="content-container">
          <div className="item">
            <Image url={itemOne.imageUrl} />
            <div className="g-type-label-strong">{itemOne.label}</div>
            <h4 className="g-type-display-4">{itemOne.title}</h4>

            <p className="g-type-body">{itemOne.description}</p>

            {itemOne.links.map((link) => (
              <div key="stable">
                <Button
                  url={link.url}
                  title={link.text}
                  linkType={link.type}
                  theme={{ variant: 'tertiary', brand }}
                />
              </div>
            ))}
          </div>
          <div className="spacer">
            <div className="vertical-spacer"></div>
            <InlineSvg className="arrow" src={ArrowIcon} />
          </div>
          <div className="item">
            <Image url={itemTwo.imageUrl} />
            <div className="g-type-label-strong">{itemTwo.label}</div>
            <h4 className="g-type-display-4">{itemTwo.title}</h4>

            <p className="g-type-body">{itemTwo.description}</p>
            {itemTwo.links.map((link) => (
              <div key="stable">
                <Button
                  key="stable"
                  url={link.url}
                  title={link.text}
                  linkType={link.type}
                  theme={{ variant: 'tertiary', brand }}
                />
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
