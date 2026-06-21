'use client';

import { Suspense, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { CheckCircle2, Loader2, MailCheck, RefreshCw, ShieldCheck } from 'lucide-react';
import { emailResendVerification, emailVerifyEmail } from '@/lib/email-auth';
import { useI18n } from '@/contexts/I18nContext';

export default function VerifyEmailPage() {
  return (
    <Suspense fallback={
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="animate-spin rounded-full h-10 w-10 border-t-2 border-b-2 border-primary" />
      </div>
    }>
      <VerifyEmailForm />
    </Suspense>
  );
}

function VerifyEmailForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { t } = useI18n();

  const initialEmail = searchParams?.get('email')?.trim() ?? '';
  const initialCode = searchParams?.get('code')?.trim() ?? '';
  const token = searchParams?.get('token')?.trim() ?? '';
  const expiresInSeconds = Number(searchParams?.get('expiresIn') ?? '0');

  const [email, setEmail] = useState(initialEmail);
  const [code, setCode] = useState(initialCode);
  const [loading, setLoading] = useState(false);
  const [resending, setResending] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [verified, setVerified] = useState(false);

  useEffect(() => {
    setEmail(initialEmail);
  }, [initialEmail]);

  const expiresLabel = useMemo(() => {
    if (!Number.isFinite(expiresInSeconds) || expiresInSeconds <= 0) {
      return null;
    }
    const hours = Math.max(1, Math.ceil(expiresInSeconds / 3600));
    return t('Current code is valid for {hours} hour(s)', { hours });
  }, [expiresInSeconds, t]);

  useEffect(() => {
    if (!token || verified) {
      return;
    }

    let cancelled = false;
    const run = async () => {
      setLoading(true);
      setError('');
      setNotice(t('Verifying email...'));
      try {
        await emailVerifyEmail({ token, email: initialEmail || undefined });
        if (cancelled) {
          return;
        }
        setVerified(true);
        setNotice(t('Email verification succeeded. Redirecting to sign-in...'));
        router.replace(`/login?verified=1&email=${encodeURIComponent(initialEmail)}`);
      } catch (err: unknown) {
        if (cancelled) {
          return;
        }
        setNotice('');
        setError(err instanceof Error ? err.message : t('Email verification failed. Please try again later.'));
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void run();
    return () => {
      cancelled = true;
    };
  }, [initialEmail, router, t, token, verified]);

  const handleVerify = async (event: React.FormEvent) => {
    event.preventDefault();
    setError('');
    setNotice('');

    if (!token && !email.trim()) {
      setError(t('Please enter your email address'));
      return;
    }
    if (!token && !code.trim()) {
      setError(t('Please enter the email verification code'));
      return;
    }

    setLoading(true);
    try {
      await emailVerifyEmail({
        email: email.trim() || undefined,
        code: code.trim() || undefined,
        token: token || undefined,
      });
      setVerified(true);
      setNotice(t('Email verification succeeded. Redirecting to sign-in...'));
      router.push(`/login?verified=1&email=${encodeURIComponent(email.trim() || initialEmail)}`);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('Email verification failed. Please try again later.'));
    } finally {
      setLoading(false);
    }
  };

  const handleResend = async () => {
    setError('');
    setNotice('');
    if (!email.trim()) {
      setError(t('Please enter your email address'));
      return;
    }

    setResending(true);
    try {
      await emailResendVerification(email.trim());
      setNotice(t('The verification code was sent again. Check your inbox or spam folder.'));
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('Failed to resend the verification code. Please try again later.'));
    } finally {
      setResending(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <div className="w-full max-w-md space-y-8">
        <div className="text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-primary">
            <MailCheck className="h-6 w-6 text-primary-foreground" />
          </div>
          <h2 className="mt-6 text-3xl font-bold tracking-tight">{t('Verify email')}</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            {t('Activate your ytb2bili account with the verification code from your email')}
          </p>
          {expiresLabel ? (
            <p className="mt-2 text-xs text-muted-foreground">{t('Current verification code: {label}', { label: expiresLabel })}</p>
          ) : null}
        </div>

        <div className="space-y-6">
          {error ? (
            <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-3 text-sm text-destructive">
              {error}
            </div>
          ) : null}

          {notice ? (
            <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">
              {notice}
            </div>
          ) : null}

          <form onSubmit={handleVerify} className="space-y-4">
            <div>
              <label htmlFor="verify-email" className="mb-1.5 block text-sm font-medium">
                {t('Email address')}
              </label>
              <input
                id="verify-email"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                placeholder="your@email.com"
                disabled={loading || Boolean(token) || verified}
                required
              />
            </div>

            <div>
              <label htmlFor="verify-code" className="mb-1.5 block text-sm font-medium">
                {t('Verification code')}
              </label>
              <input
                id="verify-code"
                inputMode="numeric"
                autoComplete="one-time-code"
                maxLength={6}
                value={code}
                onChange={(event) => setCode(event.target.value.replace(/\D/g, '').slice(0, 6))}
                className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm tracking-[0.3em] focus:outline-none focus:ring-2 focus:ring-primary"
                placeholder="123456"
                disabled={loading || verified}
                required={!token}
              />
            </div>

            <button
              type="submit"
              disabled={loading || verified}
              className="flex w-full items-center justify-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
            >
              {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <ShieldCheck className="h-4 w-4" />}
              {loading ? t('Verifying...') : t('Submit verification code')}
            </button>
          </form>

          <div className="rounded-lg border border-border bg-accent/30 p-3 text-sm text-muted-foreground">
            <p>{t('Did not receive the code? Check spam first, then resend once.')}</p>
            <button
              type="button"
              onClick={handleResend}
              disabled={resending || loading || verified}
              className="mt-3 inline-flex items-center gap-2 text-sm font-medium text-primary hover:underline disabled:opacity-50"
            >
              {resending ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
              {resending ? t('Resending...') : t('Resend code')}
            </button>
          </div>

          {verified ? (
            <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">
              <div className="flex items-center gap-2 font-medium">
                <CheckCircle2 className="h-4 w-4" />
                {t('Email verification succeeded')}
              </div>
              <p className="mt-2">{t('If the page does not redirect automatically, go to sign in to continue.')}</p>
            </div>
          ) : null}

          <p className="text-center text-sm text-muted-foreground">
            {t('Already have an account?')}{' '}
            <Link href="/login" className="font-medium text-primary hover:underline">
              {t('Back to sign in')}
            </Link>
          </p>
        </div>
      </div>
    </div>
  );
}