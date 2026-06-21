'use client';

import Image from 'next/image';
import { ReactNode, useEffect, useState } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/contexts/AuthContext';
import { useMembership } from '@/hooks/useMembership';
import { normalizeTier, TIER_META } from '@/lib/agent-models';
import { 
  ArrowUpRight,
  LayoutDashboard, 
  User, 
  X,
  Video,
  Puzzle,
  Link as LinkIcon,
  Settings,
  Rss,
  Sparkles,
  ListChecks,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { useI18n } from '@/contexts/I18nContext';
import { LanguageSwitcher } from '@/components/LanguageSwitcher';

interface NavItem {
  name: string;
  href: string;
  icon: ReactNode;
}

export default function DashboardLayout({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, currentUser, loading: authLoading } = useAuth();
  const { tier } = useMembership();
  const { t } = useI18n();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const normalizedTier = normalizeTier(tier);
  const navigation: NavItem[] = [
    { name: t('Home'), href: '/dashboard', icon: <LayoutDashboard className="h-5 w-5" /> },
    { name: t('AI Assistant'), href: '/dashboard/assistant', icon: <Sparkles className="h-5 w-5" /> },
    { name: t('Subscription'), href: '/dashboard/subscribe', icon: <Rss className="h-5 w-5" /> },
    { name: t('Video List'), href: '/dashboard/videos', icon: <Video className="h-5 w-5" /> },
    { name: t('Task Queue'), href: '/dashboard/tasks', icon: <ListChecks className="h-5 w-5" /> },
    { name: t('Account Management'), href: '/dashboard/accounts', icon: <LinkIcon className="h-5 w-5" /> },
    { name: t('Browser Extension'), href: '/dashboard/extension', icon: <Puzzle className="h-5 w-5" /> },
    { name: t('Settings'), href: '/dashboard/settings', icon: <Settings className="h-5 w-5" /> },
  ];

  // Redirect to login if auth has resolved and no user is present
  useEffect(() => {
    if (!authLoading && !user && !currentUser) {
      router.replace('/login');
    }
  }, [authLoading, user, currentUser, router]);

  // Show nothing while auth is resolving or redirecting
  if (authLoading || (!user && !currentUser)) {
    return (
      <div className="flex h-screen items-center justify-center bg-background">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-solid border-primary border-r-transparent" />
      </div>
    );
  }

  // 获取套餐标识配置
  const tierBadge = (() => {
    const badgeMeta = TIER_META[normalizedTier];

    switch (normalizedTier) {
      case 'basic':
        return {
          label: badgeMeta.label,
          className: 'bg-blue-50 text-blue-700 ring-1 ring-inset ring-blue-200',
          icon: '⚡',
        };
      case 'standard':
        return {
          label: badgeMeta.label,
          className: 'bg-cyan-50 text-cyan-700 ring-1 ring-inset ring-cyan-200',
          icon: '✨',
        };
      case 'pro':
        return {
          label: badgeMeta.label,
          className: 'bg-violet-50 text-violet-700 ring-1 ring-inset ring-violet-200',
          icon: '🚀',
        };
      case 'enterprise':
        return {
          label: badgeMeta.label,
          className: 'bg-gradient-to-r from-amber-50 to-orange-50 text-amber-800 ring-1 ring-inset ring-amber-200',
          icon: '👑',
        };
      case 'free':
      default:
        return {
          label: badgeMeta.label,
          className: 'bg-amber-50 text-amber-700 ring-1 ring-inset ring-amber-200',
          icon: '📦',
        };
    }
  })();
  const isUpgradeTier = normalizedTier === 'free';
  const membershipBadgeTitle = normalizedTier === 'free'
    ? t('You are on the free plan. Click to upgrade your membership.')
    : t('Current plan: {plan}. Click to open the membership page.', { plan: tierBadge.label });

  // 获取用户显示名称 — 优先使用后端统一返回的 display_name
  const getUserDisplayName = () => {
    if (currentUser) return currentUser.display_name;
    if (!user) return '';
    return user.displayName || user.email?.split('@')[0] || t('User');
  };

  // 获取用户详细信息
  const getUserDetails = () => {
    if (currentUser) {
      const providerLabel = currentUser.provider === 'email' ? t('Email sign-in') :
        currentUser.provider === 'firebase' ? t('Firebase sign-in') : currentUser.provider;
      return currentUser.email ? `${currentUser.email}` : providerLabel;
    }
    if (!user) return '';
    const githubProvider = user.providerData?.find(p => p.providerId === 'github.com');
    return githubProvider ? t('GitHub user') : user.email || '';
  };

  // 获取头像 URL
  const getPhotoURL = () => {
    if (currentUser?.photo_url) return currentUser.photo_url;
    return user?.photoURL || null;
  };

  return (
    <div className="flex h-screen overflow-hidden bg-background">
      {/* Mobile sidebar backdrop */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-50 w-64 transform bg-card border-r border-border transition-transform duration-300 ease-in-out lg:static lg:translate-x-0',
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        <div className="flex h-full flex-col">
          {/* Logo */}
          <div className="flex h-16 items-center justify-between border-b border-border px-6">
            <Link href="/dashboard" className="flex items-center space-x-2">
              <div className="h-8 w-8 rounded-lg bg-primary" />
              <span className="text-xl font-bold">ytb2bili</span>
            </Link>
            <button
              className="lg:hidden"
              onClick={() => setSidebarOpen(false)}
            >
              <X className="h-6 w-6" />
            </button>
          </div>

          {/* Navigation */}
          <nav className="flex-1 space-y-1 px-3 py-4">
            {navigation.map((item) => {
              const isActive = pathname === item.href;
              return (
                <Link
                  key={item.name}
                  href={item.href}
                  className={cn(
                    'flex items-center space-x-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                    isActive
                      ? 'bg-primary text-primary-foreground'
                      : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                  )}
                  onClick={() => setSidebarOpen(false)}
                >
                  {item.icon}
                  <span>{item.name}</span>
                </Link>
              );
            })}
          </nav>
        </div>
      </aside>

      {/* Main content */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Page content */}
        <main className="flex-1 overflow-y-auto bg-background p-4 lg:p-8">
          <div className="mb-4 flex justify-end lg:mb-6">
            <LanguageSwitcher />
          </div>
          {children}
        </main>
      </div>
    </div>
  );
}
