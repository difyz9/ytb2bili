'use client';

import { useCallback, useEffect, useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { buildBackendUrl } from '@/lib/backend-url';
import { getValidEmailAccessToken } from '@/lib/email-auth';
import {
  FALLBACK_MODEL_CATALOG,
  mergeModelCatalog,
  normalizeTier,
  type MembershipTier,
  type ModelCatalogItem,
} from '@/lib/agent-models';
import type { AgentModelOption } from '@/lib/api/agent';

interface AiModelCatalogResponse {
  membership_tier?: string;
  available_models?: AgentModelOption[];
}

interface UseAiModelCatalogOptions {
  onlyAvailable?: boolean;
  mergeFallback?: boolean;
}

async function fetchAiModels(
  token: string | null,
): Promise<AiModelCatalogResponse> {
  const headers: Record<string, string> = {
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const response = await fetch(buildBackendUrl('/api/agent/info'), {
    method: 'GET',
    headers,
    credentials: 'include',
    cache: 'no-store',
  });

  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error((payload as { message?: string }).message ?? `获取模型列表失败 (${response.status})`);
  }

  return ((payload as { data?: AiModelCatalogResponse }).data ?? payload) as AiModelCatalogResponse;
}

export function useAiModelCatalog(options: UseAiModelCatalogOptions = {}) {
  const { onlyAvailable = false, mergeFallback = !onlyAvailable } = options;
  const { user } = useAuth();
  const authIdentityKey = user?.uid ?? 'anonymous';
  const [models, setModels] = useState<ModelCatalogItem[]>(mergeFallback ? FALLBACK_MODEL_CATALOG : []);
  const [resolvedTier, setResolvedTier] = useState<MembershipTier>('free');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const refresh = useCallback(async () => {
    const requestIdentity = authIdentityKey;
    setLoading(true);

    try {
      const token = await getValidEmailAccessToken();
      const result = await fetchAiModels(token);
      const fetchedModels = (result.available_models ?? []).map((item) => ({
        id: item.id,
        label: item.label,
        description: item.description,
        minTier: normalizeTier(item.min_tier),
      }));
      setModels(mergeFallback ? mergeModelCatalog(fetchedModels) : fetchedModels);
      setResolvedTier(normalizeTier(result.membership_tier));
      setError(null);
    } catch (fetchError) {
      console.error('[useAiModelCatalog] fetch failed:', fetchError, { requestIdentity });
      setModels(mergeFallback ? FALLBACK_MODEL_CATALOG : []);
      setResolvedTier('free');
      setError(fetchError as Error);
    } finally {
      setLoading(false);
    }
  }, [authIdentityKey, mergeFallback]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return {
    models,
    resolvedTier,
    loading,
    error,
    refresh,
  };
}