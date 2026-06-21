import type { Metadata } from 'next';
import './globals.css';
import { AuthProvider } from '@/contexts/AuthContext';
import { I18nProvider } from '@/contexts/I18nContext';
import { Toaster } from 'react-hot-toast';
import ErrorBoundary from '@/components/ErrorBoundary';

export const metadata: Metadata = {
  title: 'ytb2bili - YouTube to Bilibili',
  description: 'Video content management platform',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <body className="font-sans antialiased">
        <I18nProvider>
          <AuthProvider>
            <ErrorBoundary>
              {children}
              <Toaster position="top-center" />
            </ErrorBoundary>
          </AuthProvider>
        </I18nProvider>
      </body>
    </html>
  );
}
