import { useState } from 'react'

import Icon from '../icon'
import style from './introduction.module.css'
import Modal from '../modal'

function Introduction({ brand, description }) {
  const [isModalShow, setIsModalShow] = useState(false)

  return (
    <div>
      <Modal show={isModalShow} close={setIsModalShow}>
        <iframe
          width="100%"
          height="100%"
          src="https://www.youtube.com/embed/mxeMdl0KvBI"
          frameborder="0"
          allowfullscreen
        ></iframe>
      </Modal>
      <p className="g-type-display-3 mt-xl mb-zero">What is {brand}?</p>
      <p className="mt-zero">{description}</p>
      <div id="btn-play-video" className={style.video}>
        <button className={style.button} onClick={() => setIsModalShow(true)}>
          <Icon icon="play" />
        </button>
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
