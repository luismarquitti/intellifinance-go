import React, { useEffect, useRef } from 'react'

interface ModalDialogProps {
  isOpen: boolean
  onClose: () => void
  labelledBy: string
  maxWidthClass?: string
  children: React.ReactNode
}

/**
 * Native `<dialog>` modal wrapper providing top-layer promotion, automatic focus containment,
 * `Esc` / back-gesture dismissal, and progressive-enhanced `closedby="any"` light-dismiss.
 */
export const ModalDialog: React.FC<ModalDialogProps> = ({
  isOpen,
  onClose,
  labelledBy,
  maxWidthClass = 'max-w-lg',
  children,
}) => {
  const dialogRef = useRef<HTMLDialogElement>(null)
  const pressedOutsideRef = useRef(false)

  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return

    if (isOpen) {
      if (!dialog.open) {
        dialog.showModal()
      }
    } else if (dialog.open) {
      dialog.close()
    }
  }, [isOpen])

  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return

    const handleClose = () => {
      onClose()
    }

    dialog.addEventListener('close', handleClose)
    return () => {
      dialog.removeEventListener('close', handleClose)
    }
  }, [onClose])

  // Light-dismiss fallback for browsers without native `closedby="any"` support (e.g. Safari)
  useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog || 'closedBy' in HTMLDialogElement.prototype) return

    const isOutside = (e: PointerEvent) => {
      const rect = dialog.getBoundingClientRect()
      return (
        e.clientX < rect.left ||
        e.clientX > rect.right ||
        e.clientY < rect.top ||
        e.clientY > rect.bottom
      )
    }

    const onPointerDown = (e: PointerEvent) => {
      pressedOutsideRef.current = isOutside(e)
    }

    const onPointerUp = (e: PointerEvent) => {
      if (pressedOutsideRef.current && isOutside(e) && dialog.open) {
        dialog.close()
      }
      pressedOutsideRef.current = false
    }

    dialog.addEventListener('pointerdown', onPointerDown)
    dialog.addEventListener('pointerup', onPointerUp)
    return () => {
      dialog.removeEventListener('pointerdown', onPointerDown)
      dialog.removeEventListener('pointerup', onPointerUp)
    }
  }, [])

  return (
    <dialog
      ref={dialogRef}
      aria-labelledby={labelledBy}
      className={`w-[calc(100vw-1.5rem)] ${maxWidthClass} rounded-2xl text-[var(--bui-text-primary)]`}
      {...({ closedby: 'any' } as Record<string, string>)}
    >
      {isOpen ? children : null}
    </dialog>
  )
}
