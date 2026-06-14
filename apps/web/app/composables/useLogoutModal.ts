// Single app-wide logout-scope chooser, opened from the top-bar dropdown and the
// mobile menu (the latter unmounts on close, so a per-component modal would
// vanish mid-action). Mirrors useAuthModal — the <LogoutModal> instance is
// mounted once in the default layout.
export const useLogoutModal = () => {
  const open = useState('kun-logout-modal-open', () => false)
  return {
    open,
    openLogoutModal: () => {
      open.value = true
    }
  }
}
