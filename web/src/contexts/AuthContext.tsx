'use client';

import { createContext, useContext, useEffect, useState, ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { apiClient } from '@/lib/api-client';
import api from '@/lib/api';
import {
  EmailAuthRequestError,
  emailLogin,
  emailGetMe,
  emailLogout,
  getValidEmailAccessToken,
  loadSession,
  loadToken,
  clearToken,
  saveSession,
  updateStoredUser,
  type UserInfoResponse,
  type LoginResponse,
} from '@/lib/email-auth';
import { translateClientText } from '@/lib/i18n';

export type { UserInfoResponse };

/** @deprecated 请使用 UserInfoResponse */
export type UnifiedUser = UserInfoResponse;
/** @deprecated 请使用 UserInfoResponse */
export type EmailUser = UserInfoResponse;

type LegacyAuthUser = {
  uid: string;
  email: string;
  displayName: string;
  photoURL: string;
  providerData: Array<{
    providerId: string;
  }>;
};

function toLegacyAuthUser(user: UserInfoResponse | null): LegacyAuthUser | null {
  if (!user) {
    return null;
  }

  return {
    uid: user.id,
    email: user.email,
    displayName: user.display_name,
    photoURL: user.photo_url,
    providerData: [],
  };
}

interface AuthContextType {
  currentUser: UserInfoResponse | null;
  /** @deprecated 请迁移到 currentUser */
  user: LegacyAuthUser | null;
  loading: boolean;
  /** @deprecated 当前项目已不启用 Firebase 登录 */
  isFirebaseConfigured: boolean;
  emailSignIn: (email: string, password: string) => Promise<LoginResponse>;
  emailSignUp: (email: string, password: string) => Promise<LoginResponse>;
  /** @deprecated 当前项目已不启用 Firebase 登录 */
  signInWithGoogle: () => Promise<never>;
  /** @deprecated 当前项目已不启用 Firebase 登录 */
  signInWithGithub: () => Promise<never>;
  emailSignOut: () => void;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [currentUser, setCurrentUser] = useState<UserInfoResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();

  useEffect(() => {
    const session = loadSession();
    const token = session?.token ?? loadToken();

    if (token) {
      if (session?.user) {
        setCurrentUser(session.user);
      }
      getValidEmailAccessToken()
        .then((validToken) => {
          if (!validToken) {
            throw new EmailAuthRequestError(401, translateClientText('Your session expired. Please sign in again.'));
          }

          apiClient.setAuthToken(validToken);
          api.setAuthToken(validToken);
          return emailGetMe(validToken);
        })
        .then((info) => {
          setCurrentUser(info);
          updateStoredUser(info);
        })
        .catch((error: unknown) => {
          if (error instanceof EmailAuthRequestError && (error.status === 401 || error.status === 403)) {
            clearToken();
            apiClient.setAuthToken(null);
            api.setAuthToken(null);
            setCurrentUser(null);
            router.push('/login');
            return;
          }

          if (!session?.user) {
            setCurrentUser(null);
          }

          console.error('Email session restore degraded:', error);
        })
        .finally(() => {
          setLoading(false);
        });
    } else {
      setLoading(false);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const emailSignIn = async (email: string, password: string): Promise<LoginResponse> => {
    const resp = await emailLogin(email, password);
    saveSession({
      token: resp.token,
      refreshToken: resp.refreshToken,
      expiresAt: resp.expires_at,
      user: resp.user,
    });
    apiClient.setAuthToken(resp.token);
    api.setAuthToken(resp.token);
    setCurrentUser(resp.user);
    return resp;
  };

  const emailSignUp = async (email: string, password: string): Promise<LoginResponse> => {
    void email;
    void password;
    throw new Error(translateClientText('Registration is disabled in local mode. Contact the administrator to create an account.'));
  };

  const emailSignOut = async () => {
    const token = loadToken();
    if (token) {
      try {
        await emailLogout(token);
      } catch (error) {
        console.error('Email logout error:', error);
      }
    }

    clearToken();
    apiClient.setAuthToken(null);
    api.setAuthToken(null);
    setCurrentUser(null);
    router.push('/login');
  };

  const logout = async () => {
    await emailSignOut();
  };

  const unsupportedFirebaseSignIn = async (): Promise<never> => {
    throw new Error(translateClientText('Firebase third-party sign-in is disabled for this project. Use email sign-in instead.'));
  };

  return (
    <AuthContext.Provider value={{
      currentUser,
      user: toLegacyAuthUser(currentUser),
      loading,
      isFirebaseConfigured: false,
      emailSignIn,
      emailSignUp,
      signInWithGoogle: unsupportedFirebaseSignIn,
      signInWithGithub: unsupportedFirebaseSignIn,
      emailSignOut,
      logout,
    }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
