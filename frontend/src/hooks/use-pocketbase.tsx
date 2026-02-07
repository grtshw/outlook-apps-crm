import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { pb, type User } from '@/lib/pocketbase'

interface AuthContext {
  user: User | null
  isLoggedIn: boolean
  isAdmin: boolean
  login: (email: string, password: string) => Promise<User>
  loginWithMicrosoft: () => Promise<void>
  logout: () => void
  refreshAuth: () => Promise<void>
}

const AuthContext = createContext<AuthContext | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(
    pb.authStore.isValid ? (pb.authStore.record as User) : null
  )

  useEffect(() => {
    const unsubscribe = pb.authStore.onChange(() => {
      setUser(pb.authStore.isValid ? (pb.authStore.record as User) : null)
    })
    return () => unsubscribe()
  }, [])

  const login = async (email: string, password: string): Promise<User> => {
    const authData = await pb.collection('users').authWithPassword(email, password)
    return authData.record as User
  }

  const loginWithMicrosoft = async (): Promise<void> => {
    await pb.collection('users').authWithOAuth2({ provider: 'microsoft' })
  }

  const logout = () => {
    pb.authStore.clear()
  }

  const refreshAuth = async (): Promise<void> => {
    try {
      await pb.collection('users').authRefresh()
    } catch {
      pb.authStore.clear()
    }
  }

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoggedIn: pb.authStore.isValid,
        isAdmin: user?.role === 'admin',
        login,
        loginWithMicrosoft,
        logout,
        refreshAuth,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
