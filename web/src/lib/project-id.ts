const FALLBACK_PROJECT_ID = 'ea8adc30a168b835d2fac68cee172433';

export const DEFAULT_PROJECT_ID = (process.env.NEXT_PUBLIC_DEFAULT_PROJECT_ID || '').trim() || FALLBACK_PROJECT_ID;

export function getDefaultProjectHeaders(): Record<string, string> {
	if (!DEFAULT_PROJECT_ID) {
		return {};
	}

	return {
		'X-Project-Id': DEFAULT_PROJECT_ID,
	};
}

export function requireDefaultProjectId(): string {
	if (!DEFAULT_PROJECT_ID) {
		throw new Error('缺少默认 projectId，无法获取项目套餐');
	}

	return DEFAULT_PROJECT_ID;
}

