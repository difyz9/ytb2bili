/**
 * 统一的 API 请求服务
 * 整合所有网络请求，提供类型安全和错误处理
 */

import axios, { AxiosInstance, AxiosError, AxiosRequestConfig } from 'axios';
import { buildProductsApiUrl, buildRemoteApiUrl, getBackendBaseUrl } from './backend-url';

// 导出 API Client
export { apiClient } from './api-client';

// ============================================================================
// 配置和类型定义
// ============================================================================

// 统一响应格式
export interface ApiResponse<T = any> {
  code: number;
  data: T;
  message: string;
}

// 分页数据
export interface PageData<T> {
  list: T[];
  total: number;
  page: number;
  size: number;
}

// Cookies 状态
export interface CookiesStatus {
  has_cookies: boolean;
  file_path?: string;
  file_size?: number;
  update_time?: string;
}

// 账号绑定相关
export interface AccountBinding {
  id: number;
  platform: string;
  platform_uid: string;
  username: string;
  avatar: string;
  status: string;
  is_active: boolean;
  is_primary?: boolean;
  last_used_at?: string;
  expires_at?: string;
  create_time: number;
}

export interface QRCodeData {
  qr_code: string;
  qr_code_key: string;
  expires_in: number;
}

// 视频相关
export interface Video {
  id: number;
  created_at: string;
  updated_at: string;
  user_id: string;
  video_id: string;
  platform: string;
  url: string;
  title: string;
  description: string;
  thumbnail: string;
  duration: number;
  status: string;
  preferred_resolution?: string;
  speech_voice_name?: string;
  retry_count: number;
  generated_title: string;
  generated_desc: string;
  generated_tags: string;
  bili_bvid: string;
  bili_aid: number;
  video_path: string;
  subtitle_path: string;
  task_steps?: unknown[];
}

export interface VideoListResponse {
  videos: Video[];
  total: number;
  page: number;
  size: number;
  total_pages: number;
}

export interface VideoTabCounts {
  all: number;
  processing: number;
  completed: number;
  failed: number;
  bili_uploaded: number;
}

// YouTube 订阅
export interface TbSubscription {
  id: number;
  created_at: string;
  updated_at: string;
  user_id: string;
  channel_id: string;
  channel_title: string;
  channel_description: string;
  channel_thumbnail_url: string;
  channel_custom_url: string;
  subscribed_at: string;
  status: string;
  synced_at: string;
}

export interface SubscriptionVideo {
  id: number;
  video_id: string;
  title: string;
  video_url: string;
  channel_id: string;
  channel_title: string;
  duration: number;
  img_url?: string;
  created_at: string;
  updated_at: string;
  subscription_id: number;
  channel_status: 'active' | 'inactive' | string;
}

// 视频处理相关
export interface TranscriptSegment {
  start: number;
  end: number;
  text: string;
}

export interface TranscriptResult {
  language: string;
  full_text: string;
  segments: TranscriptSegment[];
  srt_path?: string;
}

export interface VideoProcessData {
  video_id: string;
  video_path: string;
  audio_path?: string;
  transcript?: TranscriptResult;
  status: string;
  processed_at: string;
}

export interface VideoProcessResponse {
  success: boolean;
  message: string;
  data?: VideoProcessData;
}

export interface SystemUsageMetric {
  total_bytes: number;
  used_bytes: number;
  free_bytes: number;
  used_percent: number;
}

export interface SystemUsageResponse {
  disk_path: string;
  disk: SystemUsageMetric;
  memory: SystemUsageMetric;
  cpu_percent: number;
  uptime_seconds: number;
}

// ============================================================================
// API 客户端类
// ============================================================================

class ApiService {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: getBackendBaseUrl(),
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
      withCredentials: true, // 支持跨域 cookies
    });

    // 请求拦截器
    this.client.interceptors.request.use(
      (config) => {
        // 可以在这里添加认证 token
        const token = this.getAuthToken();
        if (token && config.headers) {
          config.headers.Authorization = `Bearer ${token}`;
        }
        return config;
      },
      (error) => Promise.reject(error)
    );

    // 响应拦截器
    this.client.interceptors.response.use(
      (response) => {
        // 自动解包统一响应格式
        if (response.data && typeof response.data === 'object') {
          if ('code' in response.data && 'data' in response.data) {
            // 统一格式: { code: 200, data: {...}, message: "success" }
            if (response.data.code === 200) {
              return { ...response, data: response.data.data };
            }
            // 业务错误
            return Promise.reject(new Error(response.data.message || '操作失败'));
          }
        }
        return response;
      },
      (error: AxiosError<ApiResponse>) => {
        // 处理 HTTP 错误
        if (error.response) {
          const message = error.response.data?.message || '请求失败';
          return Promise.reject(new Error(message));
        }
        if (error.request) {
          return Promise.reject(new Error('网络错误，请检查连接'));
        }
        return Promise.reject(error);
      }
    );
  }

  private getAuthToken(): string | null {
    if (typeof window === 'undefined') return null;
    // auth_token: set explicitly via setAuthToken (Firebase / general)
    // email_auth_token: set by email-auth.ts saveToken()
    return localStorage.getItem('auth_token') || localStorage.getItem('email_auth_token');
  }

  public setAuthToken(token: string | null) {
    if (typeof window === 'undefined') return;
    if (token) {
      localStorage.setItem('auth_token', token);
    } else {
      localStorage.removeItem('auth_token');
    }
  }

  // ============================================================================
  // Cookies 管理 API
  // ============================================================================

  async getCookiesStatus(): Promise<CookiesStatus> {
    const response = await this.client.get<CookiesStatus>('/api/v1/cookies/status');
    return response.data;
  }

  async uploadCookies(file: File): Promise<CookiesStatus> {
    const formData = new FormData();
    formData.append('file', file);

    const response = await this.client.post<CookiesStatus>('/api/v1/cookies/upload', formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
    });
    return response.data;
  }

  async deleteCookies(): Promise<void> {
    await this.client.delete('/api/v1/cookies');
  }

  // ============================================================================
  // 账号绑定 API
  // ============================================================================

  async getBindings(userId: string): Promise<AccountBinding[]> {
    const response = await this.client.get<AccountBinding[]>('/api/v1/bindings/list', {
      params: { user_id: userId },
    });
    // 后端使用统一响应格式，data字段包含实际数据
    return Array.isArray(response.data) ? response.data : [];
  }

  async generateBindingQRCode(userId: string, platform: string): Promise<QRCodeData> {
    const response = await this.client.post<QRCodeData>('/api/v1/bindings/qrcode', {
      user_id: userId,
      platform,
    });
    return response.data;
  }

  async pollBindingQRCode(authCode: string): Promise<{ status: string; binding?: AccountBinding }> {
    const response = await this.client.post('/api/v1/bindings/poll', { auth_code: authCode });
    return response.data;
  }

  async deleteBinding(bindingId: number): Promise<void> {
    await this.client.delete(`/api/v1/bindings/${bindingId}`);
  }

  async setPrimaryBinding(bindingId: number, userId: string): Promise<void> {
    await this.client.put(`/api/v1/bindings/${bindingId}/primary`, {
      user_id: userId,
    });
  }

  // ============================================================================
  // 视频管理 API
  // ============================================================================

  async getVideos(params?: {
    page?: number;
    size?: number;
    status?: string;
    user_id?: string;
    tab?: string;
  }): Promise<VideoListResponse> {
    const response = await this.client.get<VideoListResponse>('/api/v1/videos', { params });
    // Normalise: some older code paths may still return a raw array
    const data = response.data as unknown;
    if (Array.isArray(data)) {
      return { videos: data as Video[], total: (data as Video[]).length, page: 1, size: (data as Video[]).length, total_pages: 1 };
    }
    return response.data;
  }

  async getVideoCounts(params?: { user_id?: string; source_type?: string }): Promise<VideoTabCounts> {
    const response = await this.client.get<VideoTabCounts>('/api/v1/videos/counts', { params });
    return response.data;
  }

  async getVideo(id: number): Promise<Video> {
    const response = await this.client.get<Video>(`/api/v1/videos/${id}`);
    return response.data;
  }

  async createVideo(data: Partial<Video>): Promise<Video> {
    const response = await this.client.post<Video>('/api/v1/videos', data);
    return response.data;
  }

  async updateVideo(id: number, data: Partial<Video>): Promise<Video> {
    const response = await this.client.put<Video>(`/api/v1/videos/${id}`, data);
    return response.data;
  }

  async deleteVideo(id: number): Promise<void> {
    await this.client.delete(`/api/v1/videos/${id}`);
  }

  // ============================================================================
  // YouTube API
  // ============================================================================

  async getYouTubeTbSubscriptions(params?: {
    user_id?: string;
    page?: number;
    page_size?: number;
    search?: string;
    status?: 'active' | 'inactive';
  }): Promise<PageData<TbSubscription>> {
    const response = await this.client.get<PageData<TbSubscription>>('/api/youtube/TbSubscriptions', {
      params,
    });
    return response.data;
  }

  async updateYouTubeTbSubscriptionStatus(id: number, data: {
    user_id?: string;
    status?: 'active' | 'inactive';
    sync_enabled?: boolean;
  }): Promise<{ subscription: TbSubscription }> {
    const response = await this.client.patch<{ subscription: TbSubscription }>(`/api/youtube/TbSubscriptions/${id}/status`, data);
    return response.data;
  }

  async getYouTubeFeedVideos(params?: {
    user_id?: string;
    page?: number;
    pageSize?: number;
    search?: string;
    channel_status?: 'active' | 'inactive';
  }): Promise<PageData<SubscriptionVideo>> {
    const response = await this.client.get<PageData<SubscriptionVideo>>('/api/youtube/feed/videos', {
      params,
    });
    return response.data;
  }

  async refreshYouTubeFeed(): Promise<void> {
    await this.client.post('/api/youtube/feed/refresh');
  }

  // ============================================================================
  // Bilibili 账号 API
  // ============================================================================

  async getBiliQRCode(): Promise<{ qr_code_url: string; auth_code: string }> {
    const response = await this.client.get('/api/v1/bili-accounts/qrcode');
    return response.data;
  }

  async pollBiliQRCode(authCode: string): Promise<{ status: string; login_info?: any }> {
    const response = await this.client.post('/api/v1/bili-accounts/qrcode/poll', {
      auth_code: authCode,
    });
    return response.data;
  }

  async bindBiliAccount(loginInfo: any, isPrimary: boolean = false): Promise<any> {
    const response = await this.client.post('/api/v1/bili-accounts/bind', {
      login_info: loginInfo,
      is_primary: isPrimary,
    });
    return response.data;
  }

  async getBiliAccounts(): Promise<any[]> {
    const response = await this.client.get('/api/v1/bili-accounts');
    return response.data;
  }

  async unbindBiliAccount(accountId: number): Promise<void> {
    await this.client.delete(`/api/v1/bili-accounts/${accountId}`);
  }

  // ============================================================================
  // 视频处理 API
  // ============================================================================

  async uploadVideo(file: File): Promise<{ success: boolean; message: string; video_path?: string; file_name?: string }> {
    const formData = new FormData();
    formData.append('file', file);
    
    const response = await this.client.post('/api/v1/video-process/upload', formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
    });
    return response.data;
  }

  async submitVideoLink(url: string, userId?: string): Promise<VideoProcessResponse> {
    const response = await this.client.post<VideoProcessResponse>('/api/v1/video-process/submit-link', {
      url,
      user_id: userId,
    });
    return response.data;
  }

  async submitVideoFile(videoPath: string, userId?: string, title?: string): Promise<VideoProcessResponse> {
    const response = await this.client.post<VideoProcessResponse>('/api/v1/video-process/submit-video', {
      video_path: videoPath,
      user_id: userId,
      title,
    });
    return response.data;
  }

  // ============================================================================
  // 产品 API
  // ============================================================================

  async getProducts(params?: {
    projectId?: string;
    type?: string;
    status?: string;
    page?: number;
    pageSize?: number;
  }): Promise<any> {
    const response = await axios.get(buildProductsApiUrl('/products'), {
      params,
      withCredentials: true,
    });
    return response.data?.data ?? response.data;
  }

  async getProduct(productId: string): Promise<any> {
    const response = await axios.get(buildProductsApiUrl(`/products/${productId}`), {
      withCredentials: true,
    });
    return response.data?.data?.product ?? response.data?.product ?? response.data;
  }

  async getSystemUsage(): Promise<SystemUsageResponse> {
    const response = await this.client.get<SystemUsageResponse>('/api/v1/system/usage');
    return response.data;
  }

  // ============================================================================
  // 通用请求方法（用于特殊需求）
  // ============================================================================

  async get<T = any>(url: string, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.client.get<T>(url, config);
    return response.data;
  }

  async post<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.client.post<T>(url, data, config);
    return response.data;
  }

  async put<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.client.put<T>(url, data, config);
    return response.data;
  }

  async delete<T = any>(url: string, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.client.delete<T>(url, config);
    return response.data;
  }
}

// ============================================================================
// 导出单例
// ============================================================================

export const api = new ApiService();

// 便捷方法导出
export default api;
