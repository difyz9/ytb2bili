"use client";

import { useState } from 'react';
import { useI18n } from '@/contexts/I18nContext';
import { 
  Download, 
  ExternalLink, 
  CheckCircle, 
  AlertCircle,
  Puzzle
} from 'lucide-react';

type GithubReleaseAsset = {
  name: string;
  browser_download_url: string;
};

type GithubReleaseResponse = {
  assets?: GithubReleaseAsset[];
};

export default function ExtensionPage() {
  const { t } = useI18n();
  const [isDownloading, setIsDownloading] = useState(false);

  const handleDownloadExtension = async () => {
    setIsDownloading(true);
    try {
      const response = await fetch('https://api.github.com/repos/difyz9/ytb2bili_extension/releases/latest');
      const release = await response.json() as GithubReleaseResponse;
      
      if (release.assets && release.assets.length > 0) {
        const zipAsset = release.assets.find((asset) => asset.name.endsWith('.zip'));
        if (zipAsset) {
          const link = document.createElement('a');
          link.href = zipAsset.browser_download_url;
          link.download = zipAsset.name;
          document.body.appendChild(link);
          link.click();
          document.body.removeChild(link);
        } else {
          window.open('https://github.com/difyz9/ytb2bili_extension/releases/latest', '_blank');
        }
      } else {
        window.open('https://github.com/difyz9/ytb2bili_extension/releases/latest', '_blank');
      }
    } catch (error) {
      console.error('Failed to download the extension:', error);
      window.open('https://github.com/difyz9/ytb2bili_extension/releases/latest', '_blank');
    } finally {
      setIsDownloading(false);
    }
  };

  return (
      <div className="space-y-6">
        {/* 插件介绍 */}
        <div className="bg-gradient-to-r from-purple-50 to-pink-50 rounded-lg border border-purple-200 p-6">
          <div className="flex items-start space-x-4">
            <div className="flex-shrink-0">
              <Puzzle className="w-8 h-8 text-purple-600" />
            </div>
            <div className="flex-1">
              <h2 className="text-xl font-semibold text-gray-900 mb-2">
                {t('YTB2BILI browser extension')}
              </h2>
              <p className="text-gray-600 mb-4">
                {t('Install the browser extension to use YTB2BILI features more conveniently.')}
              </p>
              <div className="flex flex-col sm:flex-row gap-3">
                <button
                  onClick={handleDownloadExtension}
                  disabled={isDownloading}
                  className="flex items-center justify-center px-6 py-2 bg-purple-600 text-white rounded-md hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  {isDownloading ? (
                    <>
                      <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin mr-2"></div>
                      {t('Downloading...')}
                    </>
                  ) : (
                    <>
                      <Download className="w-4 h-4 mr-2" />
                      {t('Download latest version')}
                    </>
                  )}
                </button>
                <a
                  href="https://github.com/difyz9/ytb2bili_extension"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center justify-center px-6 py-2 border border-purple-300 text-purple-700 rounded-md hover:bg-purple-50 transition-colors"
                >
                  <ExternalLink className="w-4 h-4 mr-2" />
                  {t('GitHub project')}
                </a>
              </div>
            </div>
          </div>
        </div>

        {/* 功能特性 */}
        <div className="bg-white rounded-lg shadow-md">
          <div className="p-6 border-b border-gray-200">
            <h3 className="text-lg font-medium text-gray-900">{t('Extension features')}</h3>
          </div>
          <div className="p-6">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div className="flex items-start space-x-3">
                <CheckCircle className="w-5 h-5 text-green-500 mt-0.5 flex-shrink-0" />
                <div>
                  <h4 className="text-sm font-medium text-gray-900">{t('Auto-fetch video information')}</h4>
                  <p className="text-sm text-gray-600">{t('Automatically extract the title, description, cover, and other details from Bilibili video pages.')}</p>
                </div>
              </div>
              
              <div className="flex items-start space-x-3">
                <CheckCircle className="w-5 h-5 text-green-500 mt-0.5 flex-shrink-0" />
                <div>
                  <h4 className="text-sm font-medium text-gray-900">{t('Quick video import')}</h4>
                  <p className="text-sm text-gray-600">{t('Add the currently viewed video to the upload queue with one click.')}</p>
                </div>
              </div>
              
              <div className="flex items-start space-x-3">
                <CheckCircle className="w-5 h-5 text-green-500 mt-0.5 flex-shrink-0" />
                <div>
                  <h4 className="text-sm font-medium text-gray-900">{t('Batch operations')}</h4>
                  <p className="text-sm text-gray-600">{t('Support batch importing videos from favorites or playlists.')}</p>
                </div>
              </div>
              
              <div className="flex items-start space-x-3">
                <CheckCircle className="w-5 h-5 text-green-500 mt-0.5 flex-shrink-0" />
                <div>
                  <h4 className="text-sm font-medium text-gray-900">{t('Sync management')}</h4>
                  <p className="text-sm text-gray-600">{t('Sync data with the YTB2BILI web platform in real time.')}</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* 安装教程 */}
        <div className="bg-white rounded-lg shadow-md">
          <div className="p-6 border-b border-gray-200">
            <h3 className="text-lg font-medium text-gray-900">{t('Installation guide')}</h3>
          </div>
          <div className="p-6">
            <div className="space-y-4">
              <div className="flex items-start space-x-4">
                <div className="flex-shrink-0 w-8 h-8 bg-blue-100 text-blue-600 rounded-full flex items-center justify-center text-sm font-medium">
                  1
                </div>
                <div>
                  <h4 className="text-sm font-medium text-gray-900 mb-1">{t('Download the extension files')}</h4>
                  <p className="text-sm text-gray-600">{t('Click the "Download latest version" button above to download the extension archive locally.')}</p>
                </div>
              </div>
              
              <div className="flex items-start space-x-4">
                <div className="flex-shrink-0 w-8 h-8 bg-blue-100 text-blue-600 rounded-full flex items-center justify-center text-sm font-medium">
                  2
                </div>
                <div>
                  <h4 className="text-sm font-medium text-gray-900 mb-1">{t('Extract the files')}</h4>
                  <p className="text-sm text-gray-600">{t('Extract the downloaded zip file into a folder.')}</p>
                </div>
              </div>
              
              <div className="flex items-start space-x-4">
                <div className="flex-shrink-0 w-8 h-8 bg-blue-100 text-blue-600 rounded-full flex items-center justify-center text-sm font-medium">
                  3
                </div>
                <div>
                  <h4 className="text-sm font-medium text-gray-900 mb-1">{t('Open the extensions page')}</h4>
                  <p className="text-sm text-gray-600">{t('Open')} <code className="bg-gray-100 px-1 rounded">chrome://extensions/</code> {t('in Chrome.')}</p>
                </div>
              </div>
              
              <div className="flex items-start space-x-4">
                <div className="flex-shrink-0 w-8 h-8 bg-blue-100 text-blue-600 rounded-full flex items-center justify-center text-sm font-medium">
                  4
                </div>
                <div>
                  <h4 className="text-sm font-medium text-gray-900 mb-1">{t('Enable developer mode')}</h4>
                  <p className="text-sm text-gray-600">{t('Turn on the "Developer mode" switch in the top-right corner of the extensions page.')}</p>
                </div>
              </div>
              
              <div className="flex items-start space-x-4">
                <div className="flex-shrink-0 w-8 h-8 bg-blue-100 text-blue-600 rounded-full flex items-center justify-center text-sm font-medium">
                  5
                </div>
                <div>
                  <h4 className="text-sm font-medium text-gray-900 mb-1">{t('Load the extension')}</h4>
                  <p className="text-sm text-gray-600">{t('Click "Load unpacked" and choose the folder you extracted a moment ago.')}</p>
                </div>
              </div>
            </div>

            <div className="mt-6 p-4 bg-amber-50 border border-amber-200 rounded-lg">
              <div className="flex items-start space-x-2">
                <AlertCircle className="w-5 h-5 text-amber-600 mt-0.5 flex-shrink-0" />
                <div>
                  <h4 className="text-sm font-medium text-amber-800">{t('Notes')}</h4>
                  <p className="text-sm text-amber-700 mt-1">
                    {t('The extension is not yet listed in an app store, so manual installation is required. If you run into issues, check the detailed instructions on the GitHub project page.')}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
  );
}