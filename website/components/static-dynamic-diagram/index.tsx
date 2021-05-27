import BeforeAfterDiagram from './before-after-diagram'
import s from './style.module.css'

export default function StaticDynamicDiagram({ heading, diagrams }) {
  return (
    <div className={s.staticDynamic}>
      <h2 className={s.heading}>{heading}</h2>
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
