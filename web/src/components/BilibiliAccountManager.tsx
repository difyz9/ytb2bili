'use client';

import { useEffect, useState } from 'react';
import { useBilibiliAccounts } from '@/hooks/useBilibiliAccounts';
import { useI18n } from '@/contexts/I18nContext';
import { BilibiliBindDialog } from './BilibiliBindDialog';
import { Card } from './ui/Card';
import { Button } from './ui/Button';
import { StatusBadge } from './ui/StatusBadge';
import { Plus, Star, Trash2, Power, PowerOff, RefreshCw } from 'lucide-react';
import toast from 'react-hot-toast';

interface BilibiliAccountManagerProps {
  onAccountChange?: () => void;
}

export function BilibiliAccountManager({ onAccountChange }: BilibiliAccountManagerProps) {
  const { t, formatDate } = useI18n();
  const {
    accounts,
    loading,
    error,
    fetchAccounts,
    unbindAccount,
    setPrimary,
    toggleEnabled,
  } = useBilibiliAccounts();

  const [showBindDialog, setShowBindDialog] = useState(false);
  const [actionLoading, setActionLoading] = useState<number | null>(null);

  useEffect(() => {
    fetchAccounts();
  }, [fetchAccounts]);

  const handleUnbind = async (accountId: number) => {
    if (!confirm(t('Confirm unbinding this account?'))) return;

    setActionLoading(accountId);
    const result = await unbindAccount(accountId);
    setActionLoading(null);

    if (result.success) {
      onAccountChange?.();
    } else {
      toast.error(result.error || t('Unbind failed'));
    }
  };

  const handleSetPrimary = async (accountId: number) => {
    setActionLoading(accountId);
    const result = await setPrimary(accountId);
    setActionLoading(null);

    if (result.success) {
      onAccountChange?.();
    } else {
      toast.error(result.error || t('Failed to set the primary account'));
    }
  };

  const handleToggleEnabled = async (accountId: number, currentStatus: boolean) => {
    setActionLoading(accountId);
    const account = accounts.find(acc => acc.id === accountId);
    if (account) {
      const result = await toggleEnabled(account);
      setActionLoading(null);

      if (result.success) {
        onAccountChange?.();
      } else {
        toast.error(result.error || t('Action failed'));
      }
    } else {
      setActionLoading(null);
    }
  };

  const handleBindSuccess = () => {
    fetchAccounts();
    onAccountChange?.();
  };

  if (loading && accounts.length === 0) {
    return (
      <Card>
        <div className="flex items-center justify-center py-12">
          <div className="text-center">
            <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-current border-r-transparent" />
            <p className="mt-4 text-muted-foreground">{t('Loading...')}</p>
          </div>
        </div>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold text-gray-900 dark:text-white">
          {t('Bilibili Account Management')}
        </h2>
        <Button
          onClick={() => setShowBindDialog(true)}
          className="flex items-center space-x-2"
        >
          <Plus className="w-4 h-4" />
          <span>{t('Bind a new account')}</span>
        </Button>
      </div>

      {error && (
        <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-800 dark:text-red-200 px-4 py-3 rounded-lg">
          {error}
        </div>
      )}

      {accounts.length === 0 ? (
        <Card>
          <div className="text-center py-12">
            <div className="text-gray-400 mb-4">
              <svg
                className="mx-auto h-12 w-12"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                aria-hidden="true"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
                />
              </svg>
            </div>
            <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">
              {t('No account has been linked yet')}
            </h3>
            <p className="text-gray-500 dark:text-gray-400 mb-6">
              {t('Link a Bilibili account to start using video migration.')}
            </p>
            <Button
              onClick={() => setShowBindDialog(true)}
              className="inline-flex items-center space-x-2"
            >
              <Plus className="w-4 h-4" />
              <span>{t('Bind your first account')}</span>
            </Button>
          </div>
        </Card>
      ) : (
        <div className="grid gap-4">
          {accounts.map((account) => {
            const isExpired = account.expires_at && new Date(account.expires_at) < new Date();
            const isLoading = actionLoading === account.id;

            return (
              <Card key={account.id} className="hover:shadow-lg transition-shadow">
                <div className="flex items-center space-x-4">
                  {/* 头像 */}
                  <div className="flex-shrink-0">
                    <img
                      src={account.bili_face || 'https://static.hdslb.com/images/member/noface.gif'}
                      alt={account.bili_name}
                      className="w-16 h-16 rounded-full"
                      onError={(e) => {
                        (e.target as HTMLImageElement).src = 'https://static.hdslb.com/images/member/noface.gif';
                      }}
                    />
                  </div>

                  {/* 账号信息 */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center space-x-2 mb-1">
                      <h3 className="text-lg font-semibold text-gray-900 dark:text-white truncate">
                        {account.bili_name}
                      </h3>
                      {account.is_primary && (
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200">
                          <Star className="w-3 h-3 mr-1 fill-current" />
                          {t('Primary account')}
                        </span>
                      )}
                    </div>
                    <p className="text-sm text-gray-500 dark:text-gray-400">
                      UID: {account.bili_mid}
                    </p>
                    <div className="flex items-center space-x-4 mt-2">
                      <StatusBadge
                        status={account.is_enabled ? 'enabled' : 'disabled'}
                        text={account.is_enabled ? t('Enabled') : t('Disabled')}
                      />
                      {isExpired && (
                        <StatusBadge
                          status="error"
                          text={t('Expired')}
                        />
                      )}
                      {account.last_used_at && (
                        <span className="text-xs text-gray-400">
                          {t('Last used: {date}', { date: formatDate(account.last_used_at) })}
                        </span>
                      )}
                    </div>
                  </div>

                  {/* 操作按钮 */}
                  <div className="flex-shrink-0 flex items-center space-x-2">
                    {!account.is_primary && (
                      <Button
                        onClick={() => handleSetPrimary(account.id)}
                        variant="secondary"
                        size="sm"
                        disabled={isLoading}
                        title={t('Set as primary account')}
                      >
                        <Star className="w-4 h-4" />
                      </Button>
                    )}
                    
                    <Button
                      onClick={() => handleToggleEnabled(account.id, account.is_enabled)}
                      variant="secondary"
                      size="sm"
                      disabled={isLoading}
                      title={account.is_enabled ? t('Disable') : t('Enable')}
                    >
                      {account.is_enabled ? (
                        <PowerOff className="w-4 h-4" />
                      ) : (
                        <Power className="w-4 h-4" />
                      )}
                    </Button>

                    <Button
                      onClick={() => handleUnbind(account.id)}
                      variant="danger"
                      size="sm"
                      disabled={isLoading}
                      title={t('Unbind')}
                    >
                      {isLoading ? (
                        <RefreshCw className="w-4 h-4 animate-spin" />
                      ) : (
                        <Trash2 className="w-4 h-4" />
                      )}
                    </Button>
                  </div>
                </div>
              </Card>
            );
          })}
        </div>
      )}

      <BilibiliBindDialog
        isOpen={showBindDialog}
        onClose={() => setShowBindDialog(false)}
        onSuccess={handleBindSuccess}
        isPrimary={accounts.length === 0}
      />
    </div>
  );
}
