import axios, { AxiosInstance, AxiosError } from 'axios';
import { getBackendBaseUrl } from './backend-url';
import { resolveRequestTarget } from './auth-request-target';
import type { UserInfoResponse } from './email-auth';
import { getDefaultProjectHeaders } from '@/lib/project-id';

type RemoteAuthMeResponse = {
  user?: {
    id: string;
    email: string;
    name?: string;
    avatar?: string;
  };
};

type RemoteProfileResponse = {
  profile?: {
    id: string;
    email: string;
    name?: string;
    avatar?: string;
  };
};

class APIClient {
  private client: AxiosInstance;
  private authToken: string | null = null;

  constructor() {
    this.client = axios.create({
      baseURL: getBackendBaseUrl(),
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Request interceptor to add auth token
    this.client.interceptors.request.use(
      async (config) => {
        if (this.authToken) {
          config.headers.Authorization = `Bearer ${this.authToken}`;
        }
        return config;
      },
      (error) => Promise.reject(error)
    );

    // Response interceptor for error handling
    this.client.interceptors.response.use(
      (response) => {
        console.log('Response interceptor - original response.data:', response.data);
        
        // 处理统一响应格式: { code, data, message }
        // 如果响应包含 code 和 data 字段，解包 data
        if (response.data && typeof response.data === 'object') {
          if ('code' in response.data && 'data' in response.data) {
            console.log('Response interceptor - extracting data:', response.data.data);
            // 统一格式: { code: 200, data: {...}, message: "success" }
            // 直接修改 response.data，指向内层的 data
            response.data = response.data.data;
            console.log('Response interceptor - unwrapped data:', response.data);
          }
        }
        return response;
      },
      (error: AxiosError) => {
        if (error.response?.status === 401) {
          // Handle unauthorized
          console.error('Unauthorized request - please login');
        }
        return Promise.reject(error);
      }
    );
  }

  setAuthToken(token: string | null) {
    this.authToken = token;
  }

  // Generic HTTP methods
  async get<T = any>(url: string, config?: any): Promise<T> {
    const response = await this.client.get(url, config);
    return response.data;
  }

  async post<T = any>(url: string, data?: any, config?: any): Promise<T> {
    const response = await this.client.post(url, data, config);
    return response.data;
  }

  async put<T = any>(url: string, data?: any, config?: any): Promise<T> {
    const response = await this.client.put(url, data, config);
    return response.data;
  }

  async delete<T = any>(url: string, config?: any): Promise<T> {
    const response = await this.client.delete(url, config);
    return response.data;
  }

  // User APIs
  async getCurrentUser(): Promise<UserInfoResponse> {
    const { token, target, buildUrl } = await resolveRequestTarget();
    if (target !== 'remote' || !token) {
	    throw new Error('当前账号暂不支持资料同步，请使用邮箱账号登录后重试');
    }

    const response = await axios.get(buildUrl('/auth/me'), {
      headers: {
        Authorization: `Bearer ${token}`,
        ...getDefaultProjectHeaders(),
      },
      withCredentials: true,
    });

    const profile = (response.data?.data as RemoteAuthMeResponse | undefined)?.user
      ?? response.data?.user
      ?? (response.data?.data as RemoteProfileResponse | undefined)?.profile
      ?? response.data?.profile
      ?? response.data;

    return {
      id: profile?.id,
      display_name: profile?.name ?? '',
      email: profile?.email ?? '',
      photo_url: profile?.avatar ?? '',
      provider: 'email' as const,
    };
  }

  async updateCurrentUser(updates: { username?: string; avatar?: string }) {
    const { token, target, buildUrl } = await resolveRequestTarget();
    if (target !== 'remote' || !token) {
	    throw new Error('当前账号暂不支持资料编辑，请使用邮箱账号登录后重试');
    }

    const response = await axios.put(buildUrl('/user/profile'), {
      username: updates.username,
      avatar: updates.avatar,
    }, {
      headers: {
        Authorization: `Bearer ${token}`,
        ...getDefaultProjectHeaders(),
      },
      withCredentials: true,
    });

    return response.data?.data?.profile ?? response.data?.profile ?? response.data;
  }

  async getMyOrders() {
    const { token, target, buildUrl } = await resolveRequestTarget();
    if (target !== 'remote' || !token) {
	    throw new Error('当前账号暂不支持订单查询，请使用邮箱账号登录后重试');
    }

    const response = await axios.get(buildUrl('/orders'), {
      headers: {
        Authorization: `Bearer ${token}`,
        ...getDefaultProjectHeaders(),
      },
      withCredentials: true,
    });
    const orders = response.data?.data?.orders ?? [];
    return orders.map((order: any) => ({
      orderNo: order.orderNo,
      planID: order.productId,
      amount: order.amount,
      status: order.status,
      payWay: order.paymentMethod ?? '',
      createdAt: order.createdAt,
      paidAt: order.paidAt ?? undefined,
      extra: {
        productName: order.productName,
        productTier: order.productTier,
      },
    }));
  }

  // New Bilibili Account Management APIs (QR Code Login)
  async getBiliQRCode(userID: string = 'anonymous') {
    const response = await this.client.post('/api/v1/bindings/qrcode', {
      platform: 'bilibili',
      user_id: userID,
    });
    return response.data;
  }

  async getBiliQRCodeImage(authCode: string) {
    const response = await this.client.get(`/api/v1/bili-accounts/qrcode/image/${authCode}`, {
      responseType: 'blob',
    });
    return response.data;
  }

  async pollBiliQRCode(qrCodeKey: string) {
    const response = await this.client.post('/api/v1/bindings/poll', {
      qr_code_key: qrCodeKey,
    });
    return response.data;
  }

  async getBiliAccounts() {
    const response = await this.client.get('/api/v1/bili-accounts');
    return response.data;
  }

  async unbindBiliAccount(accountId: number) {
    const response = await this.client.delete(`/api/v1/bili-accounts/${accountId}`);
    return response.data;
  }

  async setBiliPrimary(accountId: number) {
    const response = await this.client.put(`/api/v1/bili-accounts/${accountId}/primary`);
    return response.data;
  }

  async enableBiliAccount(accountId: number) {
    const response = await this.client.put(`/api/v1/bili-accounts/${accountId}/enable`);
    return response.data;
  }

  async disableBiliAccount(accountId: number) {
    const response = await this.client.put(`/api/v1/bili-accounts/${accountId}/disable`);
    return response.data;
  }

  // YouTube Account APIs (TODO: Implement backend)
  async getYouTubeAuthURL() {
    const response = await this.client.get('/api/v1/youtube/auth/url');
    return response.data;
  }

  async bindYouTubeAccount(code: string, isPrimary: boolean = false) {
    const response = await this.client.post('/api/v1/youtube/accounts/bind', {
      code,
      is_primary: isPrimary,
    });
    return response.data;
  }

  async getYouTubeAccounts() {
    const response = await this.client.get('/api/v1/youtube/accounts');
    return response.data;
  }

  async unbindYouTubeAccount(accountId: number) {
    const response = await this.client.delete(`/api/v1/youtube/accounts/${accountId}`);
    return response.data;
  }

  async setYouTubePrimary(accountId: number) {
    const response = await this.client.put(`/api/v1/youtube/accounts/${accountId}/primary`);
    return response.data;
  }

  async enableYouTubeAccount(accountId: number) {
    const response = await this.client.put(`/api/v1/youtube/accounts/${accountId}/enable`);
    return response.data;
  }

  async disableYouTubeAccount(accountId: number) {
    const response = await this.client.put(`/api/v1/youtube/accounts/${accountId}/disable`);
    return response.data;
  }

  // YouTube Feed APIs
  async refreshYouTubeFeed() {
    const response = await this.client.get('/api/v1/youtube/feed/refresh');
    return response.data;
  }

  async getYouTubeVideos(page: number = 1, pageSize: number = 20) {
    const response = await this.client.get('/api/v1/youtube/feed/videos', {
      params: { page, pageSize },
    });
    return response.data;
  }

}

export const apiClient = new APIClient();
