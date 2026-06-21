'use client';

import { useCallback, useEffect, useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useI18n } from '@/contexts/I18nContext';
import { apiClient } from '@/lib/api-client';
import type { UserInfoResponse } from '@/lib/email-auth';
import { User, Mail, Shield } from 'lucide-react';

export default function ProfilePage() {
  const { currentUser } = useAuth();
  const { t } = useI18n();
  const [profile, setProfile] = useState<UserInfoResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState(false);
  const [formData, setFormData] = useState({
    username: '',
    avatar: '',
  });

  const fetchProfile = useCallback(async () => {
    try {
      const data = await apiClient.getCurrentUser();
      setProfile(data);
      setFormData({
        username: data.display_name || '',
        avatar: data.photo_url || '',
      });
    } catch (error) {
      console.error('Failed to fetch profile:', error);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (currentUser?.provider === 'email') {
      void fetchProfile();
      return;
    }

    if (currentUser) {
      setProfile(currentUser);
      setFormData({
        username: currentUser.display_name || '',
        avatar: currentUser.photo_url || '',
      });
      setLoading(false);
      return;
    }

    setLoading(false);
  }, [currentUser, fetchProfile]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (currentUser?.provider !== 'email') {
      return;
    }
    try {
      await apiClient.updateCurrentUser(formData);
      await fetchProfile();
      setEditing(false);
    } catch (error) {
      console.error('Failed to update profile:', error);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-current border-r-transparent" />
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      <div>
        <h2 className="text-3xl font-bold tracking-tight">{t('Profile')}</h2>
        <p className="text-muted-foreground mt-2">{t('Manage your personal information and account settings')}</p>
      </div>

      {/* Profile Card */}
      <div className="rounded-lg border border-border bg-card p-6 shadow-sm">
        <div className="flex items-start justify-between mb-6">
          <div className="flex items-center space-x-4">
            <div className="h-20 w-20 rounded-full bg-muted flex items-center justify-center">
              {profile?.photo_url ? (
                <img
                  src={profile.photo_url}
                  alt="Profile"
                  className="h-20 w-20 rounded-full object-cover"
                />
              ) : (
                <User className="h-10 w-10" />
              )}
            </div>
            <div>
              <h3 className="text-2xl font-semibold">{profile?.display_name || currentUser?.display_name || t('Nickname not set')}</h3>
              <p className="text-muted-foreground">{profile?.email}</p>
            </div>
          </div>
          <button
            onClick={() => setEditing(!editing)}
            disabled={currentUser?.provider !== 'email'}
            className="px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors"
          >
            {currentUser?.provider !== 'email' ? t('Only email accounts can be edited') : editing ? t('Cancel editing') : t('Edit profile')}
          </button>
        </div>

        {editing && currentUser?.provider === 'email' ? (
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-medium mb-2">{t('Nickname')}</label>
              <input
                type="text"
                value={formData.username}
                onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                className="w-full px-3 py-2 border border-border rounded-lg bg-background"
                placeholder={t('Enter a nickname')}
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-2">{t('Avatar URL')}</label>
              <input
                type="url"
                value={formData.avatar}
                onChange={(e) => setFormData({ ...formData, avatar: e.target.value })}
                className="w-full px-3 py-2 border border-border rounded-lg bg-background"
                placeholder="https://example.com/avatar.jpg"
              />
            </div>
            <button
              type="submit"
              className="px-6 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors"
            >
              {t('Save changes')}
            </button>
          </form>
        ) : (
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex items-center space-x-3 p-4 rounded-lg bg-muted/50">
              <Mail className="h-5 w-5 text-muted-foreground" />
              <div>
                <p className="text-sm text-muted-foreground">{t('Email address')}</p>
                <p className="font-medium">{profile?.email}</p>
              </div>
            </div>
            <div className="flex items-center space-x-3 p-4 rounded-lg bg-muted/50">
              <Shield className="h-5 w-5 text-muted-foreground" />
              <div>
                <p className="text-sm text-muted-foreground">{t('Sign-in method')}</p>
                <p className="font-medium">
                  {profile?.provider === 'email' ? t('Email sign-in') :
                   profile?.provider === 'firebase' ? t('Firebase (Google/GitHub)') :
                   profile?.provider || '-'}
                </p>
              </div>
            </div>
            {currentUser?.provider !== 'email' && (
              <div className="md:col-span-2 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
                {t('Profile editing is currently available only for email accounts.')}
              </div>
            )}
          </div>
        )}
      </div>


    </div>
  );
}
