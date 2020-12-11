import Icon from '../icon'
import style from './introduction.module.css'

function Introduction({ brand, description }) {
  return (
    <div>
      <p className="g-type-display-3 mt-xl mb-zero">What is {brand}?</p>
      <p className="mt-zero">{description}</p>
      <div className={style.video}>
        <div className={style.button}>
          <Icon icon="play" />
        </div>
        <div className={style.content}>
          <p className="g-type-display-5 mb-zero">
            Introduction to HashiCorp Consul
          </p>
          <p className="mt-zero mb-zero">Armon Dadgar</p>
          <p className="g-type-label mt-zero mb-zero">
            HashiCorp CTO and Co-founder
          </p>
        </div>
      </div>
    </div>
  )
}

export default Introduction
