'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/contexts/AuthContext';
import { useI18n } from '@/contexts/I18nContext';

export default function HomePage() {
  const router = useRouter();
  const { user, currentUser, loading } = useAuth();
  const { t } = useI18n();

  useEffect(() => {
    if (!loading) {
      if (user || currentUser) {
        router.push('/dashboard');
      } else {
        router.push('/login');
      }
    }
  }, [user, currentUser, loading, router]);

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="text-center">
        <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-current border-r-transparent align-[-0.125em] motion-reduce:animate-[spin_1.5s_linear_infinite]" />
        <p className="mt-4 text-muted-foreground">{t('Loading...')}</p>
      </div>
    </div>
  );
}
