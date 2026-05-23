'use client'

import { useState } from 'react'
import { Button, Link } from '@heroui/react'
import { useRouter } from '@bprogress/next'
import { startOAuthLogin } from '~/utils/pkce'
import { KunTextDivider } from '~/components/kun/TextDivider'
import { LogIn } from 'lucide-react'

export const LoginForm = () => {
  const router = useRouter()
  const [loading, setLoading] = useState(false)

  const handleOAuthLogin = async () => {
    setLoading(true)
    try {
      await startOAuthLogin()
    } catch {
      setLoading(false)
    }
  }

  return (
    <div className="w-72 flex flex-col gap-4">
      <p className="text-center text-sm text-default-500">
        使用 鲲 Galgame OAuth 登录以继续
      </p>

      <Button
        color="primary"
        className="w-full"
        size="lg"
        isDisabled={loading}
        isLoading={loading}
        startContent={!loading && <LogIn className="size-5" />}
        onPress={handleOAuthLogin}
      >
        鲲 Galgame OAuth 登录
      </Button>

      <KunTextDivider text="或" />

      <Button
        color="primary"
        variant="bordered"
        className="w-full"
        onPress={() => router.push('/auth/forgot')}
      >
        忘记密码
      </Button>

      <div className="flex items-center justify-center">
        <span className="mr-2 text-sm">没有 鲲 Galgame OAuth 账号?</span>
        <Link
          href={
            (process.env.NEXT_PUBLIC_KUN_OAUTH_SERVER_URL ||
              'http://127.0.0.1:9420') + '/register'
          }
          isExternal
          size="sm"
        >
          前往注册
        </Link>
      </div>
    </div>
  )
}
