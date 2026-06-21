import { buildBackendUrl } from '../backend-url';
import { getValidEmailAccessToken } from '../email-auth';

// ── Types ──────────────────────────────────────────────────────────────────────

export interface AgentTool {
  name: string;
  desc: string;
}

export interface AgentInfo {
  available: boolean;
  name?: string;
  tools?: AgentTool[];
  message?: string;
  membership_tier?: string;
  available_models?: AgentModelOption[];
}

export interface AgentModelOption {
  id: string;
  label: string;
  description: string;
  min_tier: string;
}

export interface AgentStep {
  tool: string;
  arguments: string;
  output?: string;
  error?: string;
}

export interface AgentRunResult {
  result: string;
  success: boolean;
  steps: AgentStep[];
  execution_ms: number;
}

export interface AgentErrorResponse {
  error: string;
  message: string;
}

export class AgentApiError extends Error {
  status: number;
  response: {
    status: number;
    data: {
      message?: string;
      error?: string;
      details?: unknown;
    };
  };

  constructor(status: number, message: string, details?: unknown) {
    super(message);
    this.name = 'AgentApiError';
    this.status = status;
    this.response = {
      status,
      data: {
        message,
        error: message,
        details,
      },
    };
  }
}

function buildHeaders(token: string | null, includeJson = true): HeadersInit {
  const headers: Record<string, string> = {};
  if (includeJson) {
    headers['Content-Type'] = 'application/json';
  }
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  return headers;
}

async function getRemoteTokenOrThrow(): Promise<string> {
  const token = await getValidEmailAccessToken();
  if (!token) {
    throw new AgentApiError(401, '请先登录后再使用 AI 助手。');
  }
  return token;
}

// ── API ────────────────────────────────────────────────────────────────────────

export const agentApi = {
  getInfo: async (): Promise<AgentInfo> => {
    const token = await getValidEmailAccessToken();
    if (!token) {
      return {
        available: false,
        name: 'AI 助手',
        message: '请先登录后再使用 AI 助手。',
        membership_tier: 'free',
        available_models: [],
      };
    }

    try {
      const response = await fetch(buildBackendUrl('/api/agent/info'), {
        method: 'GET',
        headers: buildHeaders(token, false),
        credentials: 'include',
        cache: 'no-store',
      });

      const payload = await response.json().catch(() => ({})) as AgentInfo;
      if (!response.ok) {
        return {
          available: false,
          name: 'AI 助手',
          message: payload.message ?? '无法连接 AI 助手服务',
          membership_tier: 'free',
          available_models: [],
        };
      }

      return {
        available: Boolean(payload.available),
        name: payload.name || 'AI 助手',
        membership_tier: payload.membership_tier ?? 'free',
        available_models: payload.available_models ?? [],
        tools: payload.tools ?? [],
        message: payload.message,
      };
    } catch (error) {
      return {
        available: false,
        name: 'AI 助手',
        message: error instanceof Error ? error.message : '无法连接 AI 助手服务',
        membership_tier: 'free',
        available_models: [],
      };
    }
  },

  run: async (query: string, model?: string, signal?: AbortSignal): Promise<AgentRunResult> => {
    const token = await getRemoteTokenOrThrow();

    const response = await fetch(buildBackendUrl('/api/agent/run'), {
      method: 'POST',
      headers: buildHeaders(token),
      credentials: 'include',
      signal,
      body: JSON.stringify({
        query,
        model,
      }),
    });

    const payload = await response.json().catch(() => ({})) as AgentRunResult & AgentErrorResponse;
    if (!response.ok) {
      const message = payload.message || payload.error || `请求失败 (${response.status})`;
      throw new AgentApiError(response.status, message, payload);
    }

	if (!payload.result) {
	  throw new AgentApiError(502, 'AI 助手未返回可解析的回复内容。', payload);
	}

	return {
	  result: payload.result,
	  success: payload.success,
	  steps: payload.steps ?? [],
	  execution_ms: payload.execution_ms ?? 0,
	};
  },
};
