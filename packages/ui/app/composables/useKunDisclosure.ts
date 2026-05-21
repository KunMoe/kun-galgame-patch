export interface UseDisclosureProps {
  isOpen?: boolean
  defaultOpen?: boolean
  onChange?: (isOpen: boolean) => void
  onOpen?: () => void
  onClose?: () => void
}

export const useKunDisclosure = (props: UseDisclosureProps = {}) => {
  const {
    isOpen: controlledIsOpen,
    defaultOpen = false,
    onChange,
    onOpen,
    onClose
  } = props

  const internalOpen = ref(defaultOpen)

  const isControlled = controlledIsOpen !== undefined

  const isOpen = computed<boolean>(() => {
    return isControlled ? controlledIsOpen! : internalOpen.value
  })

  const setOpen = (value: boolean) => {
    if (!isControlled) {
      internalOpen.value = value
    }

    onChange?.(value)

    if (value) {
      onOpen?.()
    } else {
      onClose?.()
    }
  }

  const onOpenHandler = () => {
    if (!isOpen.value) {
      setOpen(true)
    }
  }

  const onCloseHandler = () => {
    if (isOpen.value) {
      setOpen(false)
    }
  }

  const onOpenChange = () => {
    setOpen(!isOpen.value)
  }

  return {
    isOpen,
    onOpen: onOpenHandler,
    onClose: onCloseHandler,
    onOpenChange
  }
}
