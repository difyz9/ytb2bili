'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { Link as LinkIcon, Unlink, CheckCircle, AlertCircle, ExternalLink, ShieldCheck, Clock, Info, Crown } from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';
import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/ui/Modal';
import { useAuth } from '@/contexts/AuthContext';
import { useI18n } from '@/contexts/I18nContext';
import { getBackendBaseUrl } from '@/lib/backend-url';
import api, { AccountBinding } from '@/lib/api';
import toast from 'react-hot-toast';

type Platform = 'bilibili' | 'douyin' | 'xigua' | 'kuaishou' | 'youtube';

interface PlatformConfig {
  key: Platform;
  name: string;
  icon: string;
  color: string;
  description: string;
  isSupported: boolean;
}

interface GenerateQRCodeResponse {
  qr_code: string;
  qr_code_key: string;
  expires_in: number;
}

interface PollStatusResponse {
  status: 'pending' | 'scanned' | 'bound' | 'expired';
  username?: string;
  avatar?: string;
}

// 平台配置
const platformConfigs: PlatformConfig[] = [
  {
    key: 'bilibili',
    name: 'Bilibili',
    icon: '📺',
    color: '#1593c0',
    description: 'Link your Bilibili account to publish videos automatically.',
    isSupported: true,
  },
    {
    key: 'youtube',
    name: 'YouTube',
    icon: '▶️',
    color: 'bg-red-600',
    description: 'Link your YouTube account to manage international platforms together.',
    isSupported: true,
  },
  {
    key: 'douyin',
    name: 'Douyin',
    icon: '🎵',
    color: 'bg-black',
    description: 'Link your Douyin account to publish short videos automatically.',
    isSupported: false,
  },
  {
    key: 'xigua',
    name: 'Xigua Video',
    icon: '🍉',
    color: 'bg-red-500',
    description: 'Link your Xigua Video account to expand your distribution channels.',
    isSupported: false,
  },
  {
    key: 'kuaishou',
    name: 'Kuaishou',
    icon: '⚡',
    color: 'bg-orange-500',
    description: 'Link your Kuaishou account to reach more viewers.',
    isSupported: false,
  },

];

export default function BiliAccountsPage() {
  const { currentUser } = useAuth();
  const { locale, t } = useI18n();
  const [bindings, setBindings] = useState<AccountBinding[]>([]);
  const [loading, setLoading] = useState(true);
  const [showQRModal, setShowQRModal] = useState(false);
  const [bindingPlatform, setBindingPlatform] = useState<Platform | null>(null);
  const [qrCodeData, setQrCodeData] = useState<GenerateQRCodeResponse | null>(null);
  const [qrLoading, setQrLoading] = useState(false);
  const [pollingInterval, setPollingInterval] = useState<NodeJS.Timeout | null>(null);
  const [oauthWindow, setOauthWindow] = useState<Window | null>(null);
  const fetchAccountRef = useRef<() => void>(() => {});

  // 加载已绑定账号
  const fetchAccount = useCallback(async () => {
    if (!currentUser?.id) return;

    try {
      const bindings = await api.getBindings(currentUser.id);
      setBindings(bindings);
    } catch (error) {
      console.error('Failed to fetch account:', error);
      setBindings([]);
    } finally {
      setLoading(false);
    }
  }, [currentUser]);

  // 处理URL参数中的成功/错误消息
  useEffect(() => {
    if (typeof window === 'undefined') return;

    const params = new URLSearchParams(window.location.search);
    const success = params.get('success');
    const error = params.get('error');
    const message = params.get('message');
    const platform = params.get('platform');

    if (success === 'true') {
      const platformLabel = platform ? t(platformConfigs.find((item) => item.key === platform)?.name ?? platform) : t('Platform');
      const msg = message || t('{platform} account linked successfully.', { platform: platformLabel });
      toast.success(decodeURIComponent(msg));
      // 清除URL参数
      window.history.replaceState({}, '', '/dashboard/accounts');
      // 刷新账号列表
      fetchAccount();
    } else if (error) {
      toast.error(t('Linking failed: {error}', { error }));
      // 清除URL参数
      window.history.replaceState({}, '', '/dashboard/accounts');
    }
  }, [fetchAccount, t]);

  // 获取API基础URL
  const getApiBaseUrl = () => {
    return getBackendBaseUrl();
  };

  const formatBindingTime = (value?: string | number) => {
    if (!value) return t('Just linked');

    const date = typeof value === 'number' ? new Date(value * 1000) : new Date(value);
    if (Number.isNaN(date.getTime())) return t('Just linked');
    return date.toLocaleString(locale, {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  // 始终持有最新版本的 fetchAccount，避免事件监听器中的闭包陈旧问题
  useEffect(() => {
    fetchAccountRef.current = fetchAccount;
  }, [fetchAccount]);

  useEffect(() => {
    if (currentUser) {
      fetchAccount();
    }
  }, [currentUser, fetchAccount]);

  // 组件卸载时清理轮询
  useEffect(() => {
    return () => {
      if (pollingInterval) {
        clearInterval(pollingInterval);
      }
    };
  }, [pollingInterval]);

  // 监听 YouTube OAuth 弹窗回调消息
  // 使用 ref 避免 effect 因依赖变化而反复重建，防止 postMessage 在重建间隙丢失
  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      if (event.origin !== window.location.origin) return;

      if (event.data.type === 'YOUTUBE_AUTH_SUCCESS') {
        toast.success(event.data.message);
        setOauthWindow((win) => {
          if (win && !win.closed) win.close();
          return null;
        });
        setBindingPlatform(null);
        fetchAccountRef.current();
      } else if (event.data.type === 'YOUTUBE_AUTH_ERROR') {
        toast.error(t('Authorization failed: {error}', { error: event.data.error }));
        setOauthWindow((win) => {
          if (win && !win.closed) win.close();
          return null;
        });
        setBindingPlatform(null);
      }
    };

    window.addEventListener('message', handleMessage);
    return () => window.removeEventListener('message', handleMessage);
  }, [t]); // 依赖 t，确保切换语言后错误提示同步更新

  // 生成绑定二维码或OAuth URL
  const handleBindAccount = async (platform: Platform) => {
    if (!currentUser?.id) return;

    // YouTube使用OAuth流程
    if (platform === 'youtube') {
      handleYouTubeOAuth();
      return;
    }

    // 其他平台使用二维码流程
    try {
      setQrLoading(true);
      setBindingPlatform(platform);
      setShowQRModal(true);
      const data = await api.generateBindingQRCode(currentUser.id, platform);
      setQrCodeData(data);
      setQrLoading(false);
      startPolling(data.qr_code_key);
    } catch (error) {
      console.error('Failed to bind account:', error);
      setQrLoading(false);
      setShowQRModal(false);
      setBindingPlatform(null);
      toast.error(t('Failed to generate the QR code. Please try again later.'));
    }
  };

  // YouTube OAuth授权（Web弹窗方式）
  const handleYouTubeOAuth = async () => {
    if (!currentUser?.id) return;

    try {
      setBindingPlatform('youtube');
      const apiBaseUrl = getApiBaseUrl();
      const returnTo = `${window.location.origin}/dashboard/accounts/youtube-callback`;

      const response = await fetch(
        `${apiBaseUrl}/api/v1/bindings/youtube/authorize?user_id=${encodeURIComponent(currentUser.id)}&return_to=${encodeURIComponent(returnTo)}`
      );
      const data = await response.json() as { code: number; message: string; data?: { auth_url: string } };

      if (data.code === 200 && data.data?.auth_url) {
        const width = 600;
        const height = 700;
        const left = Math.round((window.screen.width - width) / 2);
        const top = Math.round((window.screen.height - height) / 2);

        const popup = window.open(
          data.data.auth_url,
          'youtube-oauth',
          `width=${width},height=${height},left=${left},top=${top},resizable=yes,scrollbars=yes`
        );

        if (popup) {
          setOauthWindow(popup);
          // 检测弹窗关闭（无论是授权完成还是用户手动关闭），兜底刷新账号列表
          const timer = setInterval(() => {
            if (popup.closed) {
              clearInterval(timer);
              setOauthWindow(null);
              setBindingPlatform(null);
              // 兜底：postMessage 已刷新时重复调用无影响，确保必然刷新
              fetchAccountRef.current();
            }
          }, 500);
        } else {
          toast.error(t('Unable to open the authorization popup. Allow pop-ups in your browser and try again.'));
          setBindingPlatform(null);
        }
      } else {
        toast.error(data.message || t('Failed to get the authorization link'));
        setBindingPlatform(null);
      }
    } catch (error) {
      console.error('Failed to start YouTube OAuth:', error);
      toast.error(t('Failed to start authorization. Please try again later.'));
      setBindingPlatform(null);
    }
  };

  // 开始轮询绑定状态
  const startPolling = (qrCodeKey: string) => {
    // 清除之前的轮询
    if (pollingInterval) {
      clearInterval(pollingInterval);
      setPollingInterval(null);
    }

    let pollCount = 0;
    const maxPolls = 100; // 最多轮询100次（300秒 = 5分钟）
    let isPolling = false; // 防止并发轮询

    const doPoll = async () => {
      if (isPolling) return; // 如果上一次轮询还没结束，跳过本次
      
      pollCount++;

      if (pollCount >= maxPolls) {
        clearInterval(interval);
        setPollingInterval(null);
        toast.error(t('The QR code expired. Generate a new one and try again.'));
        setQrCodeData(null);
        setShowQRModal(false);
        return;
      }

      isPolling = true;
      try {
        const apiBaseUrl = getApiBaseUrl();
        const response = await fetch(`${apiBaseUrl}/api/v1/bindings/poll`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ qr_code_key: qrCodeKey }),
        });

        const data = await response.json() as { code: number; message: string; data?: PollStatusResponse };

        if (data.code === 200 && data.data) {
          const status = data.data.status;

          if (status === 'bound') {
            // 绑定成功
            clearInterval(interval);
            setPollingInterval(null);
            setQrCodeData(null);
            setShowQRModal(false);
            setBindingPlatform(null);
            
            toast.success(t('{username} linked successfully.', { username: data.data.username ?? t('Account') }));
            fetchAccount(); // 重新加载账号信息
          } else if (status === 'expired') {
            // 二维码过期
            clearInterval(interval);
            setPollingInterval(null);
            toast.error(t('The QR code expired. Generate a new one and try again.'));
            setQrCodeData(null);
            setShowQRModal(false);
          }
          // status === 'pending' 时继续轮询
        }
      } catch (error) {
        console.error('轮询绑定状态失败:', error);
      } finally {
        isPolling = false;
      }
    };

    // 立即执行第一次轮询
    doPoll();

    // 每3秒轮询一次
    const interval = setInterval(doPoll, 3000);

    setPollingInterval(interval);
  };

  // 取消绑定
  const handleCancelBinding = () => {
    if (pollingInterval) {
      clearInterval(pollingInterval);
      setPollingInterval(null);
    }
    if (oauthWindow && !oauthWindow.closed) oauthWindow.close();
    setOauthWindow(null);
    setQrCodeData(null);
    setShowQRModal(false);
    setBindingPlatform(null);
  };

  // 解绑账号
  const handleUnbindAccount = async (bindingId: string, username: string) => {
    if (!confirm(t('Are you sure you want to unlink {username}?', { username }))) return;

    try {
      await api.deleteBinding(Number(bindingId));
      toast.success(t('Account unlinked successfully'));
      fetchAccount();
    } catch (error) {
      console.error('Failed to unbind account:', error);
      toast.error(t('Failed to unlink the account. Please try again later.'));
    }
  };

  const handleSetPrimaryAccount = async (bindingId: number, username: string) => {
    if (!currentUser?.id) return;

    try {
      await api.setPrimaryBinding(bindingId, currentUser.id);
      toast.success(t('{username} is now the primary account.', { username }));
      fetchAccount();
    } catch (error) {
      console.error('Failed to set primary account:', error);
      toast.error(t('Failed to set the primary account. Please try again later.'));
    }
  };

  // 获取平台配置
  const getPlatformConfig = (platform: Platform): PlatformConfig => {
    return platformConfigs.find((p) => p.key === platform) || platformConfigs[0];
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-current border-r-transparent" />
      </div>
    );
  }

  return (
    <div className="max-w-6xl mx-auto space-y-6">
      <div>
        <h2 className="text-3xl font-bold tracking-tight">{t('Account linking management')}</h2>
        <p className="text-muted-foreground mt-2">{t('Link multiple platform accounts. Bilibili supports multiple linked accounts, and you can switch the primary account for uploads.')}</p>
      </div>

      {/* 已绑定账号列表 */}
      <div className="space-y-4">
        <h3 className="text-xl font-semibold flex items-center gap-2">
          <ShieldCheck className="h-5 w-5 text-green-600" />
          {t('Linked accounts')}
        </h3>
        {loading ? (
          <div className="text-center py-12 bg-white rounded-lg border shadow-sm">
            <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-current border-r-transparent text-gray-400 mb-2" />
            <p className="text-muted-foreground text-sm">{t('Loading...')}</p>
          </div>
        ) : bindings.length === 0 ? (
          <div className="text-center py-12 bg-white rounded-lg border border-dashed shadow-sm">
            <LinkIcon className="h-12 w-12 text-gray-300 mx-auto mb-3" />
            <p className="text-muted-foreground mb-1">{t('No linked accounts yet')}</p>
            <p className="text-xs text-gray-400">{t('Choose a platform below to start linking.')}</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {bindings.map((binding) => {
              const config = getPlatformConfig(binding.platform as Platform);
              const showPrimaryAction = binding.platform === 'bilibili' && !binding.is_primary;
              const lastSyncText = formatBindingTime(binding.last_used_at ?? binding.create_time);
              return (
                <div key={binding.id} className="group relative bg-white rounded-xl border hover:border-blue-300 shadow-sm hover:shadow-md transition-all duration-300 overflow-hidden">
                   {/* 顶部装饰条 */}
                  <div className={`h-1.5 w-full ${config.color}`} />
                  
                  <div className="p-5">
                    <div className="flex justify-between items-start mb-4">
                      <div className="flex items-center gap-2">
                        <div className={`w-8 h-8 ${config.color} rounded-full flex items-center justify-center text-sm text-white shadow-sm`}>
                          {config.icon}
                        </div>
                        <span className="font-bold text-gray-900">{t(config.name)}</span>
                        {binding.is_primary && binding.platform === 'bilibili' && (
                          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-medium bg-amber-50 text-amber-700 border border-amber-100">
                            <Crown className="h-3 w-3 mr-1" />
                            {t('Primary account')}
                          </span>
                        )}
                      </div>
                      <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-green-50 text-green-700 border border-green-100">
                        <span className="w-1.5 h-1.5 rounded-full bg-green-500 mr-1.5 animate-pulse" />
                        {t('Connected')}
                      </span>
                    </div>

                    <div className="flex items-center space-x-4 mb-6">
                      <div className="relative">
                        {binding.avatar ? (
                          <img
                            src={binding.avatar}
                            alt={binding.username}
                            className="w-14 h-14 rounded-full object-cover border-2 border-white shadow-md group-hover:scale-105 transition-transform"
                            onError={(e) => {
                              e.currentTarget.style.display = 'none';
                              e.currentTarget.nextElementSibling?.classList.remove('hidden');
                            }}
                          />
                        ) : null}
                        <div
                          className={`w-14 h-14 ${config.color} rounded-full flex items-center justify-center text-2xl text-white shadow-md ${binding.avatar ? 'hidden' : ''}`}
                        >
                          {config.icon}
                        </div>
                        <div className="absolute -bottom-1 -right-1 bg-white rounded-full p-0.5 shadow-sm">
                           <CheckCircle className="h-4 w-4 text-green-500" />
                        </div>
                      </div>
                      <div className="flex-1 min-w-0">
                        <h4 className="font-semibold text-gray-900 truncate" title={binding.username}>{binding.username}</h4>
                        <p className="text-xs text-gray-500 truncate mt-0.5">UID: {binding.platform_uid}</p>
                      </div>
                    </div>

                    <div className="flex items-center justify-between pt-4 border-t border-gray-100">
                      <div className="flex flex-col">
                         <span className="text-[10px] text-gray-400">{t('Last synced')}</span>
                         <span className="text-xs font-medium text-gray-600">{lastSyncText}</span>
                      </div>
                      <div className="flex items-center gap-2">
                        {showPrimaryAction && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="text-amber-700 hover:text-amber-800 hover:bg-amber-50 h-8"
                            onClick={() => handleSetPrimaryAccount(binding.id, binding.username)}
                          >
                            <Crown className="h-3.5 w-3.5 mr-1.5" />
                            {t('Set as primary account')}
                          </Button>
                        )}
                        <Button
                          variant="ghost"
                          size="sm"
                          className="text-red-600 hover:text-red-700 hover:bg-red-50 h-8"
                          onClick={() => handleUnbindAccount(String(binding.id), binding.username)}
                        >
                          <Unlink className="h-3.5 w-3.5 mr-1.5" />
                          {t('Unlink')}
                        </Button>
                      </div>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* 可绑定平台列表 - 九宫格布局 */}
      <div className="space-y-4">
        <h3 className="text-xl font-semibold flex items-center gap-2">
           <LinkIcon className="h-5 w-5 text-blue-600" />
            {t('Add a new platform')}
        </h3>
        
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {platformConfigs.map((config) => {
              const boundCount = bindings.filter((b) => b.platform === config.key).length;
              const allowMultipleBindings = config.key === 'bilibili';
              const isBound = !allowMultipleBindings && boundCount > 0;
              return (
                <div
                  key={config.key}
                  className={`relative group bg-white rounded-xl border p-6 transition-all duration-300 ${
                    !config.isSupported 
                      ? 'opacity-70 grayscale-[0.5] hover:opacity-100 hover:grayscale-0' 
                      : 'hover:border-blue-400 hover:shadow-lg hover:-translate-y-1'
                  }`}
                >
                  <div className="flex flex-col items-center text-center space-y-4">
                    <div
                      className={`w-16 h-16 ${config.color} rounded-2xl rotate-3 group-hover:rotate-0 transition-transform duration-300 flex items-center justify-center text-3xl text-white shadow-lg`}
                    >
                      {config.icon}
                    </div>
                    <div className="flex-1 w-full">
                      <div className="flex items-center justify-center gap-2 mb-2">
                        <h3 className="font-bold text-lg text-gray-900">{t(config.name)}</h3>
                        {config.isSupported ? (
                           null
                        ) : (
                          <span className="text-[10px] bg-gray-100 text-gray-500 px-2 py-0.5 rounded-full border border-gray-200">
                            {t('In development')}
                          </span>
                        )}
                        {config.key === 'bilibili' && <span className="text-[10px] bg-pink-50 text-pink-600 px-2 py-0.5 rounded-full border border-pink-100">{t('Popular')}</span>}
                        {boundCount > 0 && (
                          <span className="text-[10px] bg-slate-50 text-slate-600 px-2 py-0.5 rounded-full border border-slate-200">
                            {t('{count} linked', { count: boundCount })}
                          </span>
                        )}
                      </div>
                      <p className="text-sm text-gray-500 mb-6 min-h-[40px] leading-relaxed">{t(config.description)}</p>
                      
                      <Button
                        onClick={() => handleBindAccount(config.key)}
                        disabled={isBound || !config.isSupported}
                        className={`w-full rounded-lg h-10 font-medium transition-all ${
                          isBound
                            ? 'bg-green-50 text-green-600 border border-green-200 hover:bg-green-50 cursor-default'
                            : !config.isSupported
                            ? 'bg-gray-100 text-gray-400 border border-gray-200'
                            : `${config.color.replace('bg-', 'bg-').replace('500', '600')} text-white hover:opacity-90 shadow-md hover:shadow-lg`
                        }`}
                      >
                        {isBound ? (
                          <span className="flex items-center justify-center">
                            <CheckCircle className="w-4 h-4 mr-1.5" /> {t('Linked')}
                          </span>
                        ) : !config.isSupported ? (
                          <span className="flex items-center justify-center">
                            <Clock className="w-4 h-4 mr-1.5" /> {t('Coming soon')}
                          </span>
                        ) : (
                          <span className="flex items-center justify-center">
                            <ExternalLink className="w-4 h-4 mr-1.5" /> {allowMultipleBindings && boundCount > 0 ? t('Link another account') : t('Link now')}
                          </span>
                        )}
                      </Button>
                    </div>
                  </div>
                </div>
              );
            })}
        </div>
      </div>

      {/* 帮助与提示 - 双栏布局 */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="bg-gradient-to-br from-blue-50 to-indigo-50 rounded-xl p-6 border border-blue-100">
          <h4 className="font-semibold text-blue-900 flex items-center gap-2 mb-4">
            <Info className="h-5 w-5 text-blue-600" />
            {t('Quick guide')}
          </h4>
          <ul className="space-y-3">
             <li className="flex items-start text-sm text-blue-800/80">
               <span className="flex-shrink-0 w-5 h-5 rounded-full bg-blue-100 text-blue-600 flex items-center justify-center text-xs font-bold mr-3 mt-0.5">1</span>
               <span>{t('Choose the platform where you want to distribute videos, then click "Link now".')}</span>
             </li>
             <li className="flex items-start text-sm text-blue-800/80">
               <span className="flex-shrink-0 w-5 h-5 rounded-full bg-blue-100 text-blue-600 flex items-center justify-center text-xs font-bold mr-3 mt-0.5">2</span>
               <span>{t('Bilibili uses QR linking, while YouTube redirects you to the Google authorization page to sign in.')}</span>
             </li>
             <li className="flex items-start text-sm text-blue-800/80">
               <span className="flex-shrink-0 w-5 h-5 rounded-full bg-blue-100 text-blue-600 flex items-center justify-center text-xs font-bold mr-3 mt-0.5">3</span>
               <span>{t('After linking succeeds, you can publish from the video list page in one click.')}</span>
             </li>
          </ul>
        </div>

        <div className="bg-gradient-to-br from-amber-50 to-orange-50 rounded-xl p-6 border border-amber-100">
          <h4 className="font-semibold text-amber-900 flex items-center gap-2 mb-4">
            <AlertCircle className="h-5 w-5 text-amber-600" />
            {t('Notes')}
          </h4>
          <ul className="space-y-2.5">
            <li className="flex items-start text-sm text-amber-800/80">
               <span className="mr-2 mt-1.5 w-1.5 h-1.5 rounded-full bg-amber-400 flex-shrink-0"></span>
              <span>{t('Bilibili QR codes are valid for 5 minutes. Complete the scan promptly.')}</span>
            </li>
            <li className="flex items-start text-sm text-amber-800/80">
               <span className="mr-2 mt-1.5 w-1.5 h-1.5 rounded-full bg-amber-400 flex-shrink-0"></span>
              <span>{t('YouTube authorization requests only the minimum publishing permissions needed for account safety.')}</span>
            </li>
            <li className="flex items-start text-sm text-amber-800/80">
               <span className="mr-2 mt-1.5 w-1.5 h-1.5 rounded-full bg-amber-400 flex-shrink-0"></span>
              <span>{t('Different platforms keep cookies for different durations. Relink the account after they expire.')}</span>
            </li>
             <li className="flex items-start text-sm text-amber-800/80">
               <span className="mr-2 mt-1.5 w-1.5 h-1.5 rounded-full bg-amber-400 flex-shrink-0"></span>
               <span>{t('Unlinking an account does not delete your historical data. You can link it again at any time.')}</span>
            </li>
          </ul>
        </div>
      </div>

      {/* 二维码弹窗 */}
      <Modal
        isOpen={showQRModal}
        onClose={handleCancelBinding}
        title={t('Scan to link {platform}', { platform: bindingPlatform ? t(getPlatformConfig(bindingPlatform).name) : '' })}
        description={t('Use the {platform} app to scan the QR code.', { platform: bindingPlatform ? t(getPlatformConfig(bindingPlatform).name) : '' })}
      >
        <div className="text-center py-8">
          {qrLoading ? (
            <div className="flex flex-col items-center justify-center py-12">
              <div className="inline-block h-12 w-12 animate-spin rounded-full border-4 border-solid border-current border-r-transparent mb-4" />
              <p className="text-muted-foreground">{t('Generating QR code...')}</p>
            </div>
          ) : qrCodeData ? (
            <div>
              <div className="inline-block p-6 bg-white border-2 border-gray-200 rounded-lg mb-6 shadow-sm">
                <QRCodeSVG
                  value={qrCodeData.qr_code}
                  size={256}
                  level="H"
                  includeMargin={true}
                  className="mx-auto"
                />
              </div>
              <div className="space-y-3">
                <p className="text-sm font-medium text-gray-900 dark:text-white">
                  {t('Use the {platform} app to scan the QR code.', { platform: bindingPlatform ? t(getPlatformConfig(bindingPlatform).name) : '' })}
                </p>
                <div className="flex items-center justify-center space-x-2 text-xs text-gray-500">
                  <AlertCircle className="h-4 w-4" />
                  <span>{t('QR code valid for {seconds} seconds', { seconds: qrCodeData.expires_in })}</span>
                </div>
                <div className="pt-4 flex justify-center">
                  <Button
                    variant="secondary"
                    onClick={handleCancelBinding}
                  >
                    {t('Cancel linking')}
                  </Button>
                </div>
              </div>
            </div>
          ) : (
            <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-current border-r-transparent" />
          )}
        </div>
      </Modal>
    </div>
  );
}
