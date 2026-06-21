'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { userSettingsApi, type UserSettingsMap } from '@/lib/api/user-settings';

const DUPLICATE_UPDATE_COOLDOWN_MS = 1000;

function buildPatchToken(patch: UserSettingsMap): string {
  return JSON.stringify(
    Object.entries(patch).sort(([leftKey], [rightKey]) => leftKey.localeCompare(rightKey)),
  );
}

export function useUserSettings(userId?: string) {
  const [settings, setSettings] = useState<UserSettingsMap>({});
  const [loading, setLoading] = useState(false);
  const loadedRef = useRef(false);
  const settingsRef = useRef<UserSettingsMap>({});
  const inFlightUpdatesRef = useRef(new Map<string, Promise<void>>());
  const recentUpdateRef = useRef(new Map<string, number>());

  useEffect(() => {
    settingsRef.current = settings;
  }, [settings]);

  useEffect(() => {
    inFlightUpdatesRef.current.clear();
    recentUpdateRef.current.clear();
  }, [userId]);

  const refresh = useCallback(async () => {
    if (!userId) {
      loadedRef.current = false;
      setSettings({});
      return {} as UserSettingsMap;
    }

    setLoading(true);
    try {
      const nextSettings = await userSettingsApi.getSettings();
      loadedRef.current = true;
      setSettings(nextSettings);
      return nextSettings;
    } finally {
      setLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const updateSettings = useCallback(async (patch: UserSettingsMap) => {
    const patchToken = buildPatchToken(patch);
    const inFlightUpdate = inFlightUpdatesRef.current.get(patchToken);
    if (inFlightUpdate) {
      await inFlightUpdate;
      return;
    }

    const lastUpdatedAt = recentUpdateRef.current.get(patchToken);
    if (lastUpdatedAt && Date.now() - lastUpdatedAt < DUPLICATE_UPDATE_COOLDOWN_MS) {
      setSettings((prev) => ({ ...prev, ...patch }));
      return;
    }

    setSettings((prev) => ({ ...prev, ...patch }));
    const request = (async () => {
      const nextSettings = await userSettingsApi.updateSettings(patch);
      setSettings((prev) => ({ ...prev, ...nextSettings }));
      recentUpdateRef.current.set(patchToken, Date.now());
    })();

    inFlightUpdatesRef.current.set(patchToken, request);

    try {
      await request;
    } finally {
      inFlightUpdatesRef.current.delete(patchToken);
    }
  }, []);

  const getSetting = useCallback((key: string) => settings[key], [settings]);

  return {
    settings,
    loading,
    loaded: loadedRef.current,
    refresh,
    updateSettings,
    getSetting,
  };
}