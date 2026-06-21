/**
 * 自动更新 API 客户端
 */

import { apiClient } from '../api-client';
import { buildBackendUrl } from '../backend-url';

export interface HealthInfo {
  ok: boolean;
  version: string;
  buildTime: string;
  commitSHA?: string;
}

export interface VersionInfo {
  version: string;
  enabled: boolean;
  autoUpdate?: boolean;
  restartOnSuccess?: boolean;
  restartDelaySeconds?: number;
  message?: string;
}

export interface UpdateCheckResult {
  hasUpdate: boolean;
  currentVersion: string;
  latestVersion: string;
  message: string;
}

export interface UpdateResult {
  updated: boolean;
  started?: boolean;
  currentVersion: string;
  latestVersion?: string;
  message: string;
  needRestart?: boolean;
}

export interface UpdateStatus {
  enabled: boolean;
  autoUpdate?: boolean;
  restartOnSuccess?: boolean;
  restartDelaySeconds?: number;
  currentVersion: string;
  updating?: boolean;
  progress?: number;
  latestVersion?: string;
  lastCheckedAt?: string;
  message: string;
}

/**
 * 从 /health 获取当前运行版本
 */
export async function getHealthInfo(): Promise<HealthInfo> {
  const response = await fetch(buildBackendUrl('/health'), {
    method: 'GET',
    credentials: 'include',
    cache: 'no-store',
  });

  if (!response.ok) {
    throw new Error(`获取健康状态失败 (${response.status})`);
  }

  return response.json();
}

/**
 * 获取当前版本信息
 */
export async function getVersion(): Promise<VersionInfo> {
  return apiClient.get('/api/v1/updater/version');
}

/**
 * 检查是否有新版本
 */
export async function checkUpdate(): Promise<UpdateCheckResult> {
  return apiClient.post('/api/v1/updater/check');
}

/**
 * 执行更新（公开接口）
 */
export async function doUpdate(): Promise<UpdateResult> {
  return apiClient.post('/api/v1/updater/update');
}

/**
 * 获取更新状态
 */
export async function getUpdateStatus(): Promise<UpdateStatus> {
  return apiClient.get('/api/v1/updater/status');
}
