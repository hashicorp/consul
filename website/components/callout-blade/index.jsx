import classNames from 'classnames'
import styles from './CalloutBlade.module.css'
import Button from '@hashicorp/react-button'
import InlineSvg from '@hashicorp/react-inline-svg'

export default function CalloutBlade({ title, callouts }) {
  return (
    <div className={styles.calloutBlade}>
      <div className={styles.contentWrapper}>
        <h3 className="g-type-display-3">{title}</h3>
        <ul
          className={classNames(styles.callouts, {
            [styles.twoUp]: callouts.length % 3 !== 0,
            [styles.threeUp]: callouts.length % 3 === 0,
          })}
        >
          {callouts.map((callout) => {
            return (
              <li key={callout.title}>
                <a className={styles.linkWrap} href={callout.link.url}>
                  <InlineSvg src={callout.icon} className={styles.icon} />
                  <div className={styles.flexWrapper}>
                    <div className={styles.infoWrapper}>
                      {callout.title && (
                        <h5 className="g-type-display-5">{callout.title}</h5>
                      )}
                      <p>{callout.description}</p>
                    </div>
                    <div className={styles.linkWrapper}>
                      <div className={styles.eyebrow}>{callout.eyebrow}</div>
                      <Button
                        title={callout.link.text}
                        url={callout.link.url}
                        linkType="inbound"
                        theme={{
                          brand: 'neutral',
                          variant: 'tertiary',
                        }}
                      />
                    </div>
                  </div>
                </a>
              </li>
            )
          })}
        </ul>
      </div>
    </div>
  )
}
