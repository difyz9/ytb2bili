import { useState, useCallback } from 'react';
import { apiClient } from '@/lib/api-client';

interface BilibiliAccount {
  id: number;
  bili_mid: number;
  bili_name: string;
  bili_face: string;
  is_enabled: boolean;
  is_primary: boolean;
  expires_at?: string;
  last_used_at?: string;
  created_at: string;
  updated_at: string;
}

export function useBilibiliAccounts() {
  const [accounts, setAccounts] = useState<BilibiliAccount[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchAccounts = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await apiClient.getBiliAccounts();
      setAccounts(data.accounts || []);
    } catch (err) {
      setError('获取账号列表失败');
      console.error('Failed to fetch Bilibili accounts:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  const unbindAccount = useCallback(async (accountId: number) => {
    try {
      await apiClient.unbindBiliAccount(accountId);
      await fetchAccounts();
      return { success: true };
    } catch (err) {
      console.error('Failed to unbind account:', err);
      return { success: false, error: '解绑账号失败' };
    }
  }, [fetchAccounts]);

  const setPrimary = useCallback(async (accountId: number) => {
    try {
      await apiClient.setBiliPrimary(accountId);
      await fetchAccounts();
      return { success: true };
    } catch (err) {
      console.error('Failed to set primary:', err);
      return { success: false, error: '设置主账号失败' };
    }
  }, [fetchAccounts]);

  const toggleEnabled = useCallback(async (account: BilibiliAccount) => {
    try {
      if (account.is_enabled) {
        await apiClient.disableBiliAccount(account.id);
      } else {
        await apiClient.enableBiliAccount(account.id);
      }
      await fetchAccounts();
      return { success: true };
    } catch (err) {
      console.error('Failed to toggle account:', err);
      return { success: false, error: '操作失败' };
    }
  }, [fetchAccounts]);

  return {
    accounts,
    loading,
    error,
    fetchAccounts,
    unbindAccount,
    setPrimary,
    toggleEnabled,
  };
}
