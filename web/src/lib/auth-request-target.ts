import { buildBackendUrl, buildRemoteApiUrl } from './backend-url';
import { getValidEmailAccessToken } from './email-auth';

type UrlBuilder = (path: string) => string;

export interface RequestTargetContext {
  token: string | null;
  target: 'backend' | 'remote';
  buildUrl: UrlBuilder;
}

export async function resolveRequestTarget(options?: {
  allowRemoteAnonymous?: boolean;
}): Promise<RequestTargetContext> {
  const emailToken = await getValidEmailAccessToken();
  if (emailToken) {
    return {
      token: emailToken,
      target: 'remote',
      buildUrl: buildRemoteApiUrl,
    };
  }

  if (options?.allowRemoteAnonymous) {
    return {
      token: null,
      target: 'remote',
      buildUrl: buildRemoteApiUrl,
    };
  }

  return {
    token: null,
    target: 'backend',
    buildUrl: (path: string) => buildBackendUrl(`/api${path}`),
  };
}