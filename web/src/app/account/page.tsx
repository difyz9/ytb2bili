'use client';

import { useI18n } from '@/contexts/I18nContext';
import { useAuth } from '@/contexts/AuthContext';
import { useMembership } from '@/hooks/useMembership';
import OrderHistory from '@/components/membership/OrderHistory';
import { formatDateForLocale } from '@/lib/i18n';
import { Crown, TrendingUp, Calendar, Package } from 'lucide-react';
import Link from 'next/link';

export default function AccountPage() {
  const { user, currentUser } = useAuth();
  const { locale, t } = useI18n();
  const { tier, quota, membershipData, loading } = useMembership();

  if (!user && !currentUser) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
        <div className="text-center">
          <p className="text-gray-600 dark:text-gray-400 mb-4">{t('Please sign in first')}</p>
          <Link
            href="/login"
            className="inline-block px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
          >
            {t('Go to sign in')}
          </Link>
        </div>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
        <div className="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-blue-600"></div>
      </div>
    );
  }

  const getTierName = (tier: string) => {
    const tierNames: Record<string, string> = {
      free: t('Free user'),
      basic: t('Basic membership'),
      pro: t('Pro membership'),
      enterprise: t('Enterprise membership'),
    };
    return tierNames[tier] || tier;
  };

  const getTierColor = (tier: string) => {
    const colors: Record<string, string> = {
      free: 'from-gray-500 to-gray-600',
      basic: 'from-blue-500 to-blue-600',
      pro: 'from-purple-500 to-purple-600',
      enterprise: 'from-amber-500 to-amber-600',
    };
    return colors[tier] || 'from-gray-500 to-gray-600';
  };

  const isExpired = membershipData?.membership?.expiresAt 
    ? new Date(membershipData.membership.expiresAt) < new Date()
    : false;

  const daysRemaining = membershipData?.membership?.expiresAt
    ? Math.max(0, Math.ceil((new Date(membershipData.membership.expiresAt).getTime() - Date.now()) / (1000 * 60 * 60 * 24)))
    : 0;

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 py-8 px-4">
      <div className="max-w-6xl mx-auto space-y-6">
        {/* Page Header */}
        <div className="flex items-center justify-between">
          <h1 className="text-3xl font-bold text-gray-900 dark:text-white">{t('Account center')}</h1>
        </div>


        {/* Order History */}
        <div className="bg-white dark:bg-gray-800 rounded-2xl p-6 shadow-lg border border-gray-200 dark:border-gray-700">
          <OrderHistory />
        </div>
      </div>
    </div>
  );
}
