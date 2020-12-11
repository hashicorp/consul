import Icon from '../icon'
import style from './card.module.css'

function Card({ type, icon, url, brand, children }) {
  return (
    <a href={url} className={style.card} data-brand={brand}>
      <div>
        {icon != null ? <Icon icon={icon} brand={brand} /> : null}
        {type != null ? <p className={style.label}>{type}</p> : null}
        <p className="g-type-body-small-strong mt-s mb-zero">{children}</p>
      </div>
    </a>
  )
}

export default Card
