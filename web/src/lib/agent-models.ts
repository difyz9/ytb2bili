export type MembershipTier = 'free' | 'basic' | 'standard' | 'pro' | 'enterprise';

export interface TierMeta {
  label: string;
  shortLabel: string;
  accentClassName: string;
  description: string;
}

export interface ModelCatalogItem {
  id: string;
  label: string;
  description: string;
  minTier: MembershipTier;
  modelId?: string;
  provider?: string;
  creditCostPerRequest?: number | null;
  isFeatured?: boolean;
  locked?: boolean;
}

export interface WorkerAiModel {
  key?: string;
  id: string;
  name: string;
  modelId: string;
  provider?: string | null;
  modelType?: string | null;
  minTier?: string | null;
  creditCostPerRequest?: number | null;
  isFeatured?: boolean | null;
  description?: string | null;
  locked?: boolean;
  inputPricing?: number;
  outputPricing?: number;
}

export const TIER_ORDER: MembershipTier[] = ['free', 'basic', 'standard', 'pro', 'enterprise'];

export const TIER_META: Record<MembershipTier, TierMeta> = {
  free: {
    label: '免费版',
    shortLabel: 'Free',
    accentClassName: 'border-gray-200 bg-gray-50 text-gray-700 dark:border-gray-800 dark:bg-gray-900/40 dark:text-gray-200',
    description: '适合基础问答和轻量操作。',
  },
  basic: {
    label: '基础会员',
    shortLabel: 'Basic',
    accentClassName: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900 dark:bg-blue-950/30 dark:text-blue-200',
    description: '开放更强的对话推理能力。',
  },
  standard: {
    label: '标准会员',
    shortLabel: 'Standard',
    accentClassName: 'border-cyan-200 bg-cyan-50 text-cyan-700 dark:border-cyan-900 dark:bg-cyan-950/30 dark:text-cyan-200',
    description: '适合更高频使用和进阶内容生成。',
  },
  pro: {
    label: 'PRO 会员',
    shortLabel: 'Pro',
    accentClassName: 'border-violet-200 bg-violet-50 text-violet-700 dark:border-violet-900 dark:bg-violet-950/30 dark:text-violet-200',
    description: '适合高质量内容生成和复杂任务。',
  },
  enterprise: {
    label: '高级会员',
    shortLabel: 'Enterprise',
    accentClassName: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200',
    description: '解锁完整模型能力和更高性能。',
  },
};

export const FALLBACK_MODEL_CATALOG: ModelCatalogItem[] = [
  {
    id: 'gpt-4o-mini',
    label: 'GPT-4o Mini',
    description: '适合日常问答和轻量任务',
    minTier: 'free',
  },
  {
    id: 'claude-3-5-haiku-20241022',
    label: 'Claude 3.5 Haiku',
    description: '速度快、成本低，适合高频对话。',
    minTier: 'basic',
  },
  {
    id: 'gemini-3.1-flash-image-preview',
    label: 'Gemini 3.1 Flash',
    description: '多模态理解更强，适合图文混合任务。',
    minTier: 'standard',
  },
  {
    id: 'gpt-4o',
    label: 'GPT-4o',
    description: '综合能力强，适合高质量生成与复杂问答。',
    minTier: 'pro',
  },
  {
    id: 'claude-3-opus-20240229',
    label: 'Claude 3 Opus',
    description: '复杂推理与长文档处理能力最强。',
    minTier: 'enterprise',
  },
];

export const MODEL_CATALOG = FALLBACK_MODEL_CATALOG;

export function normalizeTier(tier?: string | null): MembershipTier {
  if (tier === 'basic' || tier === 'standard' || tier === 'pro' || tier === 'enterprise') {
    return tier;
  }
  return 'free';
}

export function toModelCatalogItem(model: WorkerAiModel): ModelCatalogItem {
  return {
    id: model.id || model.key || model.modelId,
    label: model.name,
    description: model.description?.trim() || `${model.provider ?? 'AI'} 模型`,
    minTier: normalizeTier(model.minTier),
    modelId: model.modelId,
    provider: model.provider ?? undefined,
    creditCostPerRequest: model.creditCostPerRequest ?? null,
    isFeatured: model.isFeatured ?? undefined,
    locked: model.locked,
  };
}

export function mergeModelCatalog(models: ModelCatalogItem[], fallback: ModelCatalogItem[] = FALLBACK_MODEL_CATALOG): ModelCatalogItem[] {
  const merged = new Map<string, ModelCatalogItem>(fallback.map((item) => [item.id, item]));

  for (const model of models) {
    const prev = merged.get(model.id);
    merged.set(model.id, {
      ...prev,
      ...model,
      description: model.description || prev?.description || 'AI 模型',
    });
  }

  return Array.from(merged.values()).sort((left, right) => {
    const tierDelta = TIER_ORDER.indexOf(left.minTier) - TIER_ORDER.indexOf(right.minTier);
    if (tierDelta !== 0) {
      return tierDelta;
    }
    return left.label.localeCompare(right.label, 'zh-CN');
  });
}

export function modelsForTier(models: ModelCatalogItem[], tier?: string | null): ModelCatalogItem[] {
  const currentTier = normalizeTier(tier);
  const currentIndex = TIER_ORDER.indexOf(currentTier);

  return models.filter((model) => TIER_ORDER.indexOf(model.minTier) <= currentIndex);
}

export function lockedModelsForTier(models: ModelCatalogItem[], tier?: string | null): ModelCatalogItem[] {
  const unlockedIds = new Set(modelsForTier(models, tier).map((model) => model.id));
  return models.filter((model) => !unlockedIds.has(model.id));
}

export function nextTier(tier?: string | null): MembershipTier | null {
  const currentTier = normalizeTier(tier);
  const currentIndex = TIER_ORDER.indexOf(currentTier);
  return TIER_ORDER[currentIndex + 1] ?? null;
}

export function highestTier(...tiers: Array<string | null | undefined>): MembershipTier {
  return tiers.reduce<MembershipTier>((currentHighest, candidate) => {
    const normalizedCandidate = normalizeTier(candidate);
    return TIER_ORDER.indexOf(normalizedCandidate) > TIER_ORDER.indexOf(currentHighest)
      ? normalizedCandidate
      : currentHighest;
  }, 'free');
}