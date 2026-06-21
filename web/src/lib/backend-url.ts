function trimTrailingSlash(value: string): string {
  return value.replace(/\/$/, '');
}

const DEFAULT_PRODUCTS_API_URL = 'https://open.ytb2bili.com/api/v1';

function isLocalhostUrl(value: string): boolean {
  try {
    const url = new URL(value);
    return url.hostname === 'localhost' || url.hostname === '127.0.0.1';
  } catch {
    return false;
  }
}

export const apiBaseConfig = {
  get backend() {
    const configured = process.env.NEXT_PUBLIC_API_URL?.trim();
    if (configured) {
      // Production deployments can be served from arbitrary domains. Ignore a baked-in
      // localhost override when the current page is already running on a real domain.
      if (typeof window !== 'undefined') {
        const currentOrigin = trimTrailingSlash(window.location.origin);
        if (!isLocalhostUrl(currentOrigin) && isLocalhostUrl(configured)) {
          return currentOrigin;
        }
      }

      return trimTrailingSlash(configured);
    }

    if (typeof window !== 'undefined') {
      return trimTrailingSlash(window.location.origin);
    }

    return '';
  },
  get remote() {
    return trimTrailingSlash(
      process.env.NEXT_PUBLIC_REMOTE_API_URL?.trim() || DEFAULT_PRODUCTS_API_URL,
    );
  },
} as const;

export function getBackendBaseUrl(): string {
  return apiBaseConfig.backend;
}

export function getRemoteApiBaseUrl(): string {
  return apiBaseConfig.remote;
}

export function getProductsApiBaseUrl(): string {
  return trimTrailingSlash(
    process.env.NEXT_PUBLIC_PRODUCTS_API_URL?.trim() || DEFAULT_PRODUCTS_API_URL,
  );
}

export function buildBackendUrl(path: string): string {
  const baseUrl = getBackendBaseUrl();
  if (!path.startsWith('/')) {
    return `${baseUrl}/${path}`;
  }

  return `${baseUrl}${path}`;
}

export function buildRemoteApiUrl(path: string): string {
  const baseUrl = getRemoteApiBaseUrl();
  if (!path.startsWith('/')) {
    return `${baseUrl}/${path}`;
  }

  return `${baseUrl}${path}`;
}

export function buildProductsApiUrl(path: string): string {
  const baseUrl = getProductsApiBaseUrl();
  if (!path.startsWith('/')) {
    return `${baseUrl}/${path}`;
  }

  return `${baseUrl}${path}`;
}