// Global login/register modal — one <AuthLoginModal> is mounted in the default
// layout and bound to `isOpen`. Anything that needs login opens that SAME modal
// (the one the top-bar 登录 button shows) instead of redirecting to home or
// firing a "请先登录" toast, so the login prompt is uniform across the site.
//
// Usage:
//   - imperative (click handlers):  if (!requireLogin()) return
//   - direct control (buttons):     const { open } = useAuthModal()
//   - page gate (whole routes):     wrap content in <AuthRequired>
export const useAuthModal = () => {
  // useState → SSR-safe, shared singleton across every component instance.
  const isOpen = useState('kun-auth-modal-open', () => false)
  const userStore = useUserStore()

  const open = () => {
    isOpen.value = true
  }
  const close = () => {
    isOpen.value = false
  }

  // Guard for login-required actions. Returns true when the user is logged in;
  // otherwise opens the login modal and returns false — call as the first line
  // of a handler: `if (!requireLogin()) return`.
  const requireLogin = (): boolean => {
    if (userStore.isLoggedIn) {
      return true
    }
    open()
    return false
  }

  return { isOpen, open, close, requireLogin }
}
