'use client';

import Link from 'next/link';
import { useI18n } from '@/contexts/I18nContext';
import { Mail } from 'lucide-react';

export default function RegisterPage() {
  const { t } = useI18n();

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <div className="w-full max-w-md space-y-8">
        {/* Header */}
        <div className="text-center">
          <div className="mx-auto h-12 w-12 rounded-xl bg-primary flex items-center justify-center">
            <Mail className="h-6 w-6 text-primary-foreground" />
          </div>
          <h2 className="mt-6 text-3xl font-bold tracking-tight">{t('Create your account')}</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            {t('Self-service registration is disabled in local mode')}
          </p>
        </div>

        <div className="space-y-6">
          <div className="rounded-lg border border-border bg-accent/40 p-4 text-sm text-muted-foreground">
            {t('This deployment uses local-config accounts only. Ask the administrator to add your email and password in the backend config file.')}
          </div>

          {/* Login link */}
          <p className="text-center text-sm text-muted-foreground">
            {t('Already have an account?')}{' '}
            <Link href="/login" className="font-medium text-primary hover:underline">
              {t('Sign in now')}
            </Link>
          </p>
        </div>
      </div>
    </div>
  );
}
