'use client';

import { useState, useEffect, useCallback } from 'react';
import { useI18n } from '@/contexts/I18nContext';
import { formatDateForLocale } from '@/lib/i18n';
import { Upload, CheckCircle, XCircle, AlertCircle, Trash2, Info } from 'lucide-react';
import api, { CookiesStatus } from '@/lib/api';

export default function CookiesManager() {
  const { locale, t } = useI18n();
  const [status, setStatus] = useState<CookiesStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error' | 'info'; text: string } | null>(null);

  // 加载 cookies 状态
  const loadStatus = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.getCookiesStatus();
      setStatus(data);
    } catch (error) {
      console.error('Failed to load cookies status:', error);
      setMessage({ type: 'error', text: error instanceof Error ? t(error.message) : t('Failed to load cookies status') });
    } finally {
      setLoading(false);
    }
  }, [t]);

  // 上传文件
  const handleUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;

    // 验证文件类型
    if (!file.name.endsWith('.txt')) {
      setMessage({ type: 'error', text: t('Only .txt cookies files are supported') });
      return;
    }

    // 验证文件大小 (10MB)
    if (file.size > 10 * 1024 * 1024) {
      setMessage({ type: 'error', text: t('The file size must not exceed 10MB') });
      return;
    }

    setUploading(true);
    setMessage(null);

    try {
      const data = await api.uploadCookies(file);
      setMessage({ type: 'success', text: t('Cookies file uploaded successfully!') });
      setStatus(data);
    } catch (error) {
      console.error('Failed to upload cookies file:', error);
      setMessage({ 
        type: 'error', 
        text: error instanceof Error ? t(error.message) : t('Upload failed. Please try again later.') 
      });
    } finally {
      setUploading(false);
      // 清空文件选择
      event.target.value = '';
    }
  };

  // 删除文件
  const handleDelete = async () => {
    if (!confirm(t('Are you sure you want to delete the cookies file?'))) {
      return;
    }

    setLoading(true);
    setMessage(null);

    try {
      await api.deleteCookies();
      setMessage({ type: 'success', text: t('Cookies file deleted') });
      setStatus({ has_cookies: false });
    } catch (error) {
      console.error('Failed to delete cookies file:', error);
      setMessage({ 
        type: 'error', 
        text: error instanceof Error ? t(error.message) : t('Delete failed. Please try again later.') 
      });
    } finally {
      setLoading(false);
    }
  };

  // 格式化文件大小
  const formatFileSize = (bytes?: number) => {
    if (!bytes) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
  };

  useEffect(() => {
    loadStatus();
  }, [loadStatus]);

  return (
    <div className="space-y-6">
      {/* 说明信息 */}
      <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
        <div className="flex items-start space-x-3">
          <Info className="w-5 h-5 text-blue-600 flex-shrink-0 mt-0.5" />
          <div className="flex-1 text-sm text-blue-800">
            <p className="font-medium mb-2">{t('Why are cookies required?')}</p>
            <p className="mb-2">
              {t('YouTube now requires authentication to reduce bot access. After you upload a cookies file, yt-dlp can download restricted videos.')}
            </p>
            <a
              href="https://github.com/yt-dlp/yt-dlp/wiki/FAQ#how-do-i-pass-cookies-to-yt-dlp"
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-600 hover:text-blue-800 underline"
            >
              {t('See how to export YouTube cookies')} {'->'}
            </a>
          </div>
        </div>
      </div>

      {/* 消息提示 */}
      {message && (
        <div
          className={`rounded-lg p-4 ${
            message.type === 'success'
              ? 'bg-green-50 border border-green-200'
              : message.type === 'error'
              ? 'bg-red-50 border border-red-200'
              : 'bg-blue-50 border border-blue-200'
          }`}
        >
          <div className="flex items-center space-x-3">
            {message.type === 'success' && <CheckCircle className="w-5 h-5 text-green-600" />}
            {message.type === 'error' && <XCircle className="w-5 h-5 text-red-600" />}
            {message.type === 'info' && <AlertCircle className="w-5 h-5 text-blue-600" />}
            <span
              className={`text-sm ${
                message.type === 'success'
                  ? 'text-green-800'
                  : message.type === 'error'
                  ? 'text-red-800'
                  : 'text-blue-800'
              }`}
            >
              {message.text}
            </span>
          </div>
        </div>
      )}

      {/* Cookies 状态 */}
      {loading ? (
        <div className="text-center py-8">
          <div className="inline-block w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full animate-spin"></div>
        </div>
      ) : status ? (
        <div className="bg-white border border-gray-200 rounded-lg p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-medium text-gray-900">{t('Cookies file status')}</h3>
            {status.has_cookies ? (
              <span className="flex items-center space-x-2 text-green-600">
                <CheckCircle className="w-5 h-5" />
                <span className="text-sm font-medium">{t('Configured')}</span>
              </span>
            ) : (
              <span className="flex items-center space-x-2 text-gray-400">
                <XCircle className="w-5 h-5" />
                <span className="text-sm font-medium">{t('Not configured')}</span>
              </span>
            )}
          </div>

          {status.has_cookies && (
            <div className="space-y-3 mb-4">
              <div className="flex justify-between text-sm">
                <span className="text-gray-600">{t('File size:')}</span>
                <span className="text-gray-900 font-medium">{formatFileSize(status.file_size)}</span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-gray-600">{t('Updated at:')}</span>
                <span className="text-gray-900 font-medium">{status.update_time ? formatDateForLocale(locale, status.update_time, { dateStyle: 'medium', timeStyle: 'short' }) : '-'}</span>
              </div>
            </div>
          )}

          <div className="flex space-x-3">
            {/* 上传按钮 */}
            <label className="flex-1">
              <input
                type="file"
                accept=".txt"
                onChange={handleUpload}
                disabled={uploading}
                className="hidden"
              />
              <div
                className={`flex items-center justify-center space-x-2 px-4 py-2 border border-gray-300 rounded-lg cursor-pointer transition-colors ${
                  uploading
                    ? 'bg-gray-100 cursor-not-allowed'
                    : 'hover:bg-gray-50 hover:border-gray-400'
                }`}
              >
                <Upload className="w-4 h-4" />
                <span className="text-sm font-medium">
                  {uploading ? t('Uploading...') : status.has_cookies ? t('Update cookies') : t('Upload cookies')}
                </span>
              </div>
            </label>

            {/* 删除按钮 */}
            {status.has_cookies && (
              <button
                onClick={handleDelete}
                disabled={loading}
                className="px-4 py-2 border border-red-300 text-red-600 rounded-lg hover:bg-red-50 hover:border-red-400 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <Trash2 className="w-4 h-4" />
              </button>
            )}
          </div>
        </div>
      ) : null}

      {/* 使用说明 */}
      <div className="bg-gray-50 border border-gray-200 rounded-lg p-4">
        <h4 className="font-medium text-gray-900 mb-3">{t('How do I export cookies?')}</h4>
        <ol className="space-y-2 text-sm text-gray-700">
          <li className="flex items-start space-x-2">
            <span className="font-medium text-gray-900">1.</span>
            <span>
              {t('Install the browser extension')} {' '}
              <a
                href="https://chrome.google.com/webstore/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc"
                target="_blank"
                rel="noopener noreferrer"
                className="text-blue-600 hover:underline"
              >
                Get cookies.txt LOCALLY
              </a>
            </span>
          </li>
          <li className="flex items-start space-x-2">
            <span className="font-medium text-gray-900">2.</span>
            <span>{t('Sign in to YouTube in your browser')}</span>
          </li>
          <li className="flex items-start space-x-2">
            <span className="font-medium text-gray-900">3.</span>
            <span>{t('Visit youtube.com and click the extension icon to export cookies')}</span>
          </li>
          <li className="flex items-start space-x-2">
            <span className="font-medium text-gray-900">4.</span>
            <span>{t('Upload the exported cookies.txt file here')}</span>
          </li>
        </ol>
      </div>
    </div>
  );
}
