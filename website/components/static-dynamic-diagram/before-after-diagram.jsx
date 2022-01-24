import Image from '@hashicorp/react-image'
import InlineSvg from '@hashicorp/react-inline-svg'
import alertIcon from 'public/img/static-dynamic-diagram/alert.svg?include'
import checkIcon from 'public/img/static-dynamic-diagram/check.svg?include'
import s from './before-after-diagram.module.css'

export default function BeforeAfterDiagram({
  beforeHeadline,
  beforeContent,
  beforeImage,
  afterHeadline,
  afterContent,
  afterImage,
}) {
  return (
    <div className={s.beforeAfterDiagram}>
      <div className={s.beforeSide}>
        <div className={s.image}>
          <div>
            <Image {...beforeImage} />
          </div>
        </div>
        <div className={s.contentContainer}>
          <span className={s.iconLineContainer}>
            <InlineSvg className={s.beforeIcon} src={alertIcon} />
            <span className={s.lineSegment} />
          </span>
          <div>
            {beforeHeadline && (
              <h3
                className={s.contentHeadline}
                dangerouslySetInnerHTML={{
                  __html: beforeHeadline,
                }}
              />
            )}
            {beforeContent && (
              <div
                className={s.beforeContent}
                dangerouslySetInnerHTML={{
                  __html: beforeContent,
                }}
              />
            )}
          </div>
        </div>
      </div>
      <div className={s.afterSide}>
        <div className={s.image}>
          <div>
            <Image {...afterImage} />
          </div>
        </div>
        <div className={s.contentContainer}>
          <span className={s.iconLineContainer}>
            <InlineSvg className={s.afterIcon} src={checkIcon} />
          </span>
          <div>
            {afterHeadline && (
              <h3
                className={s.contentHeadline}
                dangerouslySetInnerHTML={{
                  __html: afterHeadline,
                }}
              />
            )}
            {afterContent && (
              <div
                className={s.afterContent}
                dangerouslySetInnerHTML={{
                  __html: afterContent,
                }}
              />
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
