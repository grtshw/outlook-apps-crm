import { useState } from 'react'
import { useNavigate } from 'react-router'
import { toast } from 'sonner'
import { useAuth } from '@/hooks/use-pocketbase'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader } from '@/components/ui/card'

export function LoginPage() {
  const navigate = useNavigate()
  const { loginWithMicrosoft, login, isLoggedIn } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [showPasswordForm, setShowPasswordForm] = useState(false)

  if (isLoggedIn) {
    navigate('/', { replace: true })
    return null
  }

  const handleMicrosoftLogin = async () => {
    setIsLoading(true)
    try {
      await loginWithMicrosoft()
      navigate('/')
    } catch {
      toast.error('Failed to sign in with Microsoft')
    } finally {
      setIsLoading(false)
    }
  }

  const handlePasswordLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    try {
      await login(email, password)
      navigate('/')
    } catch {
      toast.error('Invalid email or password')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-brand-black p-4">
      <Card className="w-full max-w-sm bg-brand-black border-white/10">
        <CardHeader className="text-center pb-2">
          <img
            src="/images/logo-white.svg"
            alt="CRM"
            className="h-8 mx-auto mb-6"
          />
          <h1 className="text-xl text-white">Sign in to CRM</h1>
        </CardHeader>
        <CardContent className="space-y-4">
          <Button
            onClick={handleMicrosoftLogin}
            disabled={isLoading}
            className="w-full bg-white text-brand-black hover:bg-white/90"
          >
            <svg className="w-5 h-5 mr-2" viewBox="0 0 21 21" fill="none">
              <rect x="1" y="1" width="9" height="9" fill="#F25022" />
              <rect x="11" y="1" width="9" height="9" fill="#7FBA00" />
              <rect x="1" y="11" width="9" height="9" fill="#00A4EF" />
              <rect x="11" y="11" width="9" height="9" fill="#FFB900" />
            </svg>
            Sign in with Microsoft
          </Button>

          {!showPasswordForm ? (
            <button
              onClick={() => setShowPasswordForm(true)}
              className="w-full text-sm text-white/60 hover:text-white"
            >
              Sign in with password instead
            </button>
          ) : (
            <form onSubmit={handlePasswordLogin} className="space-y-3">
              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <span className="w-full border-t border-white/10" />
                </div>
                <div className="relative flex justify-center text-xs">
                  <span className="bg-brand-black px-2 text-white/60">or</span>
                </div>
              </div>
              <Input
                type="email"
                placeholder="Email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                className="bg-white/5 border-white/10 text-white placeholder:text-white/40"
              />
              <Input
                type="password"
                placeholder="Password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                className="bg-white/5 border-white/10 text-white placeholder:text-white/40"
              />
              <Button
                type="submit"
                disabled={isLoading}
                className="w-full"
              >
                Sign in
              </Button>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
