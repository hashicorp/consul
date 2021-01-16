import Icon from '../icon'
import s from './modal.module.css'

function Modal({ show, close, children }) {
  if (!show) return null
  return (
    <div className={s.container}>
      <div className={s.scrim} onClick={() => close(false)}></div>
      <div className={s.contentsContainer} onClick={() => close(false)}>
        <div className={s.grid}>
          <div className={s.contents}>{children}</div>
          <button className={s.close} onClick={() => close(false)}>
            <Icon icon="closeRefined" />
          </button>
        </div>
      </div>
    </div>
  )
}

export default Modal
