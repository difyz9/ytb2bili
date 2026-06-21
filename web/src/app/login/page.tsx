'use client';

import { Suspense, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useAuth } from '@/contexts/AuthContext';
import { useI18n } from '@/contexts/I18nContext';
import { LogIn, Mail, Github, Eye, EyeOff } from 'lucide-react';

// Default export wraps LoginForm in Suspense – required by Next.js App Router
// because useSearchParams() opts the component out of static rendering.
export default function LoginPage() {
  return (
    <Suspense fallback={
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="animate-spin rounded-full h-10 w-10 border-t-2 border-b-2 border-primary" />
      </div>
    }>
      <LoginForm />
    </Suspense>
  );
}

function LoginForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { signInWithGoogle, signInWithGithub, emailSignIn, isFirebaseConfigured } = useAuth();
  const { t } = useI18n();
  const verificationNotice = searchParams?.get('verified') === '1'
    ? t('Your email was verified. Sign in with your email and password.')
    : '';
  const presetEmail = searchParams?.get('email') ?? '';

  const [email, setEmail] = useState(presetEmail);
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  // Respect an optional ?redirect= query param so protected pages (e.g.
  // /membership) can send users back after a successful login instead of
  // always landing on /dashboard.
  const getRedirectPath = (): string => {
    const redirect = searchParams?.get('redirect');
    if (redirect && redirect.startsWith('/')) return redirect;
    return '/dashboard';
  };

  const handleEmailSignIn = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await emailSignIn(email, password);
      router.push(getRedirectPath());
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('Sign in failed. Please check your email and password.'));
    } finally {
      setLoading(false);
    }
  };


  const handleGoogleSignIn = async () => {
    setError('');
    setLoading(true);
    try {
      await signInWithGoogle();
      router.push(getRedirectPath());
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('Google sign-in failed'));
    } finally {
      setLoading(false);
    }
  };

  const handleGithubSignIn = async () => {
    setError('');
    setLoading(true);
    try {
      await signInWithGithub();
      router.push(getRedirectPath());
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('GitHub sign-in failed'));
    } finally {
      setLoading(false);
    }
  };

  const busy = loading;

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <div className="w-full max-w-md space-y-8">
        {/* Header */}
        <div className="text-center">
          <div className="mx-auto h-12 w-12 rounded-xl bg-primary flex items-center justify-center">
            <Mail className="h-6 w-6 text-primary-foreground" />
          </div>
          <h2 className="mt-6 text-3xl font-bold tracking-tight">
            {t('Log in to ytb2bili')}
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            {t('Use email or a third-party account to continue')}
          </p>
        </div>

        <div className="space-y-6">
          {verificationNotice && (
            <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">
              {verificationNotice}
            </div>
          )}

          {error && (
            <div className="rounded-lg bg-destructive/10 border border-destructive/20 p-3 text-sm text-destructive">
              {error}
            </div>
          )}

          <form onSubmit={handleEmailSignIn} className="space-y-4">
            <div>
              <label htmlFor="email" className="block text-sm font-medium mb-1.5">
                {t('Email address')}
              </label>
              <input
                id="email"
                type="email"
                autoComplete="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full px-3 py-2 border border-border rounded-lg bg-background focus:outline-none focus:ring-2 focus:ring-primary text-sm"
                placeholder="your@email.com"
              />
            </div>

            <div>
              <label htmlFor="password" className="block text-sm font-medium mb-1.5">
                {t('Password')}
              </label>
              <div className="relative">
                <input
                  id="password"
                  type={showPassword ? 'text' : 'password'}
                  autoComplete="current-password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full px-3 py-2 pr-10 border border-border rounded-lg bg-background focus:outline-none focus:ring-2 focus:ring-primary text-sm"
                  placeholder="••••••••"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((v) => !v)}
                  className="absolute inset-y-0 right-0 flex items-center pr-3 text-muted-foreground hover:text-foreground"
                  tabIndex={-1}
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>

              <p className="mt-2 text-xs text-muted-foreground">
                {t('Password reset is disabled in local mode. Contact the administrator if you need to change your password.')}
              </p>
            </div>

            <button
              type="submit"
              disabled={busy}
              className="w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors disabled:opacity-50 font-medium text-sm"
            >
              <LogIn className="h-4 w-4" />
              {loading ? t('Signing in...') : t('Email sign-in')}
            </button>
          </form>

          {isFirebaseConfigured && (
            <>
              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t border-border" />
                </div>
                <div className="relative flex justify-center text-xs">
                  <span className="bg-background px-2 text-muted-foreground">
                    {t('Or sign in using')}
                  </span>
                </div>
              </div>

              <div className="space-y-3">
                <button
                  type="button"
                  onClick={handleGoogleSignIn}
                  disabled={busy}
                  className="w-full flex items-center justify-center gap-2 px-4 py-2.5 border border-border rounded-lg hover:bg-accent transition-colors disabled:opacity-50 text-sm font-medium"
                >
                  <svg className="h-4 w-4 shrink-0" viewBox="0 0 24 24">
                    <path fill="currentColor" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" />
                    <path fill="currentColor" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" />
                    <path fill="currentColor" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" />
                    <path fill="currentColor" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" />
                  </svg>
                  {t('Continue with Google')}
                </button>
                <button
                  type="button"
                  onClick={handleGithubSignIn}
                  disabled={busy}
                  className="w-full flex items-center justify-center gap-2 px-4 py-2.5 border border-border rounded-lg hover:bg-accent transition-colors disabled:opacity-50 text-sm font-medium"
                >
                  <Github className="h-4 w-4 shrink-0" />
                  {t('Continue with GitHub')}
                </button>
              </div>
            </>
          )}

          <p className="text-center text-sm text-muted-foreground">
            {t('Need an account? Contact the administrator to add your email in local auth config.')}
          </p>
        </div>
      </div>
    </div>
  );
}
