'use client';

import { useState, useEffect, useCallback } from 'react';
import Image from 'next/image';
import { Modal } from './ui/Modal';
import { Button } from './ui/Button';
import { useI18n } from '@/contexts/I18nContext';
import { apiClient } from '@/lib/api-client';
import { X, RefreshCw, CheckCircle, AlertCircle } from 'lucide-react';

interface BilibiliBindDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: () => void;
  isPrimary?: boolean;
}

type QRCodeStatus = 'loading' | 'ready' | 'scanning' | 'success' | 'expired' | 'error';

export function BilibiliBindDialog({ 
  isOpen, 
  onClose, 
  onSuccess,
}: BilibiliBindDialogProps) {
  const { t } = useI18n();
  const [qrCodeUrl, setQrCodeUrl] = useState<string>('');
  const [qrCodeKey, setQrCodeKey] = useState<string>('');
  const [status, setStatus] = useState<QRCodeStatus>('loading');
  const [message, setMessage] = useState<string>('');
  const [pollingInterval, setPollingInterval] = useState<NodeJS.Timeout | null>(null);

  const fetchQRCode = useCallback(async () => {
    setStatus('loading');
    setMessage(t('Generating QR code...'));
    
    try {
      // TODO: 从AuthContext获取真实用户ID
      const response = await apiClient.getBiliQRCode('anonymous');
      
      if (response.code === 0 && response.data) {
        setQrCodeUrl(response.data.qr_code);
        setQrCodeKey(response.data.qr_code_key);
        setStatus('ready');
        setMessage(t('Use the {platform} app to scan the QR code.', { platform: 'Bilibili' }));
      } else {
        setStatus('error');
        setMessage(response.message || t('Failed to generate the QR code. Please try again later.'));
      }
    } catch (error) {
      console.error('Failed to fetch QR code:', error);
      setStatus('error');
      setMessage(t('Failed to generate the QR code. Please try again later.'));
    }
  }, [t]);

  const pollQRCodeStatus = useCallback(async () => {
    if (!qrCodeKey) return;

    try {
      const response = await apiClient.pollBiliQRCode(qrCodeKey);
      
      if (response.code === 0 && response.data) {
        const { status: bindingStatus, username } = response.data;
        
        switch (bindingStatus) {
          case 'bound': // 绑定成功
            setStatus('success');
            setMessage(
              username
                ? `${t('Binding succeeded!')} ${t('Welcome, {name}', { name: username })}`
                : t('Binding succeeded!')
            );
            
            // 停止轮询
            if (pollingInterval) {
              clearInterval(pollingInterval);
              setPollingInterval(null);
            }

            // 延迟关闭对话框
            setTimeout(() => {
              onSuccess?.();
              onClose();
            }, 1500);
            break;

          case 'pending': // 未扫描或待确认
            if (status !== 'scanning') {
              setStatus('ready');
              setMessage(t('Use the {platform} app to scan the QR code.', { platform: 'Bilibili' }));
            }
            break;

          case 'expired': // 二维码已失效
            setStatus('expired');
            setMessage(t('The QR code expired. Generate a new one and try again.'));
            if (pollingInterval) {
              clearInterval(pollingInterval);
              setPollingInterval(null);
            }
            break;

          default:
            console.log('Unknown binding status:', bindingStatus);
        }
      }
    } catch (error) {
      console.error('Failed to poll QR code status:', error);
      // 轮询失败不影响继续轮询
    }
  }, [qrCodeKey, status, pollingInterval, onSuccess, onClose, t]);

  useEffect(() => {
    if (isOpen) {
      fetchQRCode();
    } else {
      // 关闭对话框时清理
      if (pollingInterval) {
        clearInterval(pollingInterval);
        setPollingInterval(null);
      }
      setQrCodeUrl('');
      setQrCodeKey('');
      setStatus('loading');
      setMessage('');
    }
  }, [isOpen, fetchQRCode, pollingInterval]);

  useEffect(() => {
    if (qrCodeKey && (status === 'ready' || status === 'scanning') && !pollingInterval) {
      // 开始轮询，2秒间隔
      const interval = setInterval(pollQRCodeStatus, 2000);
      setPollingInterval(interval);
      return () => clearInterval(interval);
    }
  }, [qrCodeKey, status, pollingInterval, pollQRCodeStatus]);

  const handleRefresh = () => {
    if (pollingInterval) {
      clearInterval(pollingInterval);
      setPollingInterval(null);
    }
    fetchQRCode();
  };

  const getStatusIcon = () => {
    switch (status) {
      case 'success':
        return <CheckCircle className="w-12 h-12 text-green-500 mx-auto mb-4" />;
      case 'error':
      case 'expired':
        return <AlertCircle className="w-12 h-12 text-red-500 mx-auto mb-4" />;
      default:
        return null;
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose}>
      <div className="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-md w-full">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
            {t('Bind Bilibili account')}
          </h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-500 dark:hover:text-gray-300"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        <div className="text-center py-6">
          {getStatusIcon()}

          {status === 'loading' && (
            <div className="flex flex-col items-center">
              <div className="inline-block h-12 w-12 animate-spin rounded-full border-4 border-solid border-blue-600 border-r-transparent mb-4" />
              <p className="text-gray-600 dark:text-gray-300">{message}</p>
            </div>
          )}

          {(status === 'ready' || status === 'scanning') && qrCodeUrl && (
            <div className="space-y-4">
              <div className="bg-white p-4 rounded-lg inline-block shadow-md">
                <Image
                  src={qrCodeUrl} 
                  alt={t('Bilibili login QR code')} 
                  width={256}
                  height={256}
                  unoptimized
                  className="w-64 h-64 mx-auto"
                />
              </div>
              <div className="space-y-2">
                <p className={`font-medium ${
                  status === 'scanning' ? 'text-blue-600' : 'text-gray-900 dark:text-white'
                }`}>
                  {message}
                </p>
                {status === 'scanning' && (
                  <div className="flex items-center justify-center space-x-2 text-blue-600">
                    <div className="w-2 h-2 bg-blue-600 rounded-full animate-pulse" />
                    <div className="w-2 h-2 bg-blue-600 rounded-full animate-pulse delay-75" />
                    <div className="w-2 h-2 bg-blue-600 rounded-full animate-pulse delay-150" />
                  </div>
                )}
              </div>
            </div>
          )}

          {status === 'success' && (
            <div className="space-y-2">
              <p className="text-green-600 font-medium">{message}</p>
            </div>
          )}

          {(status === 'error' || status === 'expired') && (
            <div className="space-y-4">
              <p className="text-red-600 font-medium">{message}</p>
              <Button onClick={handleRefresh} className="mx-auto flex items-center space-x-2">
                <RefreshCw className="w-4 h-4" />
                <span>{t('Refresh QR code')}</span>
              </Button>
            </div>
          )}
        </div>

        <div className="mt-6 flex justify-end space-x-3">
          <Button
            onClick={onClose}
            variant="secondary"
            disabled={status === 'success'}
          >
            {t('Cancel')}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
