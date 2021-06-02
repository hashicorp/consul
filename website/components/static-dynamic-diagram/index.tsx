import BeforeAfterDiagram from './before-after-diagram'
import s from './style.module.css'

export default function StaticDynamicDiagram({
  heading,
  description,
  diagrams,
}) {
  return (
    <div className={s.staticDynamic}>
      <div className={s.content}>
        <h2 className={s.heading}>{heading}</h2>
        {description && <p className={s.description}>{description}</p>}
      </div>
      <BeforeAfterDiagram
        {...diagrams}
        beforeImage={{
          format: 'png',
          url: '/img/static-dynamic-diagram/consul_static_isometric@2x.png',
        }}
        afterImage={{
          format: 'png',
          url: '/img/static-dynamic-diagram/consul_dynamic_isometric@2x.png',
        }}
      />
    </div>
  )
}
