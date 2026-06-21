'use client';

import { useCallback, useEffect, useState } from 'react';
import { Download, RefreshCw, AlertCircle } from 'lucide-react';
import { useI18n } from '@/contexts/I18nContext';
import { checkUpdate, doUpdate, getHealthInfo, getUpdateStatus, getVersion } from '@/lib/api/updater';
import { toast } from 'react-hot-toast';

function getErrorMessage(error: unknown): string | null {
  if (error instanceof Error) {
    return error.message;
  }
  return null;
}

export default function UpdateManager() {
  const { t } = useI18n();
  const [checking, setChecking] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [hasUpdate, setHasUpdate] = useState(false);
  const [currentVersion, setCurrentVersion] = useState('');
  const [latestVersion, setLatestVersion] = useState('');
  const [restartOnSuccess, setRestartOnSuccess] = useState(false);
  const [restartDelaySeconds, setRestartDelaySeconds] = useState(0);

  const loadStatus = useCallback(async () => {
    const [healthInfo, versionInfo, statusInfo] = await Promise.all([getHealthInfo(), getVersion(), getUpdateStatus()]);
    setCurrentVersion(healthInfo.version || '');
    setUpdating(Boolean(statusInfo.updating));
    setRestartOnSuccess(Boolean(statusInfo.restartOnSuccess ?? versionInfo.restartOnSuccess));
    setRestartDelaySeconds(statusInfo.restartDelaySeconds ?? versionInfo.restartDelaySeconds ?? 0);
    if (statusInfo.latestVersion) {
      setLatestVersion(statusInfo.latestVersion);
    }
  }, []);

  useEffect(() => {
    void loadStatus().catch(() => {
      // ignore initial load failure; keep manual actions available
    });
  }, [loadStatus]);

  useEffect(() => {
    if (!updating) {
      return;
    }

    const timer = window.setInterval(() => {
      void getUpdateStatus()
        .then((status) => {
          setUpdating(Boolean(status.updating));
          setRestartOnSuccess(Boolean(status.restartOnSuccess));
          setRestartDelaySeconds(status.restartDelaySeconds ?? 0);
          if (status.latestVersion) {
            setLatestVersion(status.latestVersion);
          }

          if (!status.updating && (status.progress ?? 0) >= 100) {
            setHasUpdate(false);
            toast.success(status.message ? t(status.message) : t('Update completed. Please restart the app.'));
            void loadStatus().catch(() => {
              // ignore refresh failure after update completes
            });
          }
        })
        .catch(() => {
          // keep previous state if polling fails
        });
    }, 1500);

    return () => window.clearInterval(timer);
  }, [loadStatus, t, updating]);

  const handleCheckUpdate = async () => {
    setChecking(true);
    try {
      const result = await checkUpdate();
      setLatestVersion(result.latestVersion);
      setHasUpdate(result.hasUpdate);
      
      if (result.hasUpdate) {
        toast.success(t('New version found: {version}', { version: result.latestVersion }));
      } else {
        toast.success(t(result.message));
      }
    } catch (error: unknown) {
      const errorMessage = getErrorMessage(error);
      toast.error(errorMessage ? t(errorMessage) : t('Failed to check for updates'));
    } finally {
      setChecking(false);
    }
  };

  const handleDoUpdate = async () => {
    if (!hasUpdate) {
      toast.error(t('You are already on the latest version'));
      return;
    }

    const confirmed = window.confirm(
      t('Do you want to update to the latest version?\nThe app must restart after the update to take effect.')
    );

    if (!confirmed) return;

    setUpdating(true);
    try {
      const result = await doUpdate();

      if (result.started) {
        toast.success(t(result.message));
      } else {
        toast.success(t(result.message));
        setUpdating(false);
      }
    } catch (error: unknown) {
      const errorMessage = getErrorMessage(error);
      toast.error(errorMessage ? t(errorMessage) : t('Update failed'));
      setUpdating(false);
    } finally {
      void loadStatus().catch(() => {
        // ignore status refresh failure after action
      });
    }
  };

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
            {t('System update')}
          </h2>
          {currentVersion && (
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              {t('Current version: {version}', { version: currentVersion })}
            </p>
          )}
        </div>
      </div>


      {/* 更新状态卡片 */}
      {hasUpdate && (
        <div className="mb-6 p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
          <div className="flex items-start gap-3">
            <AlertCircle className="w-5 h-5 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0" />
            <div className="flex-1">
              <h3 className="text-sm font-semibold text-blue-900 dark:text-blue-100">
                {t('New version available')}
              </h3>
              <p className="text-sm text-blue-700 dark:text-blue-300 mt-1">
                {t('Latest version: {version}', { version: latestVersion })}
              </p>
              <p className="text-xs text-blue-600 dark:text-blue-400 mt-2">
                {t('Update to the latest version for the best experience and newest features')}
              </p>
            </div>
          </div>
        </div>
      )}

      {/* 操作按钮 */}
      <div className="flex gap-3">
        <button
          onClick={handleCheckUpdate}
          disabled={checking || updating}
          className="flex items-center gap-2 px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-200 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <RefreshCw className={`w-4 h-4 ${checking ? 'animate-spin' : ''}`} />
          {checking ? t('Checking...') : t('Check for updates')}
        </button>

        {hasUpdate && (
          <button
            onClick={handleDoUpdate}
            disabled={updating}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Download className={`w-4 h-4 ${updating ? 'animate-bounce' : ''}`} />
            {updating ? t('Update in progress...') : t('Update now')}
          </button>
        )}
      </div>

      {/* 更新说明 */}
      <div className="mt-6 p-4 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
        <h4 className="text-sm font-medium text-gray-900 dark:text-white mb-2">
          {t('Update notes')}
        </h4>
        <ul className="text-xs text-gray-600 dark:text-gray-400 space-y-1">
          <li>• {t('The update process may take a few minutes. Please wait patiently.')}</li>
          <li>• {restartOnSuccess ? t('The current configuration will automatically restart the app {seconds} seconds after the update completes', { seconds: restartDelaySeconds }) : t('The app must restart after the update to take effect')}</li>
          <li>• {t('If the update fails, the system automatically rolls back to the current version')}</li>
          <li>• {t('Updating during off-peak hours is recommended')}</li>
        </ul>
      </div>
    </div>
  );
}
