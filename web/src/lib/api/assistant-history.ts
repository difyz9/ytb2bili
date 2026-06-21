import type { AgentStep } from './agent';

export const DEFAULT_ASSISTANT_CONVERSATION_ID = 'default';

const ASSISTANT_HISTORY_DB_NAME = 'ytb2bili-assistant-history';
const ASSISTANT_HISTORY_DB_VERSION = 1;
const ASSISTANT_HISTORY_STORE_NAME = 'conversation_history';
const ASSISTANT_HISTORY_FALLBACK_PREFIX = 'ytb2bili:assistant-history';

export interface AssistantHistoryMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: string;
  steps?: AgentStep[];
  execution_ms?: number;
  success?: boolean;
  isError?: boolean;
  videoId?: string;
  biliBvid?: string;
  trackingStatus?: 'pending' | 'tracking' | 'completed' | 'failed';
}

export interface AssistantHistoryPayload {
  conversationId: string;
  updatedAt: string | null;
  messages: AssistantHistoryMessage[];
}

interface AssistantHistoryRecord extends AssistantHistoryPayload {
  storageKey: string;
}

function buildStorageKey(userId: string, conversationId: string): string {
  return `${userId}::${conversationId}`;
}

function buildFallbackStorageKey(storageKey: string): string {
  return `${ASSISTANT_HISTORY_FALLBACK_PREFIX}:${storageKey}`;
}

function getEmptyPayload(conversationId: string): AssistantHistoryPayload {
  return {
    conversationId,
    updatedAt: null,
    messages: [],
  };
}

function canUseBrowserStorage(): boolean {
  return typeof window !== 'undefined';
}

function openAssistantHistoryDb(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    if (!canUseBrowserStorage() || !('indexedDB' in window)) {
      reject(new Error('indexedDB unavailable'));
      return;
    }

    const request = window.indexedDB.open(ASSISTANT_HISTORY_DB_NAME, ASSISTANT_HISTORY_DB_VERSION);

    request.onerror = () => {
      reject(request.error ?? new Error('failed to open indexedDB'));
    };

    request.onupgradeneeded = () => {
      const db = request.result;
      if (!db.objectStoreNames.contains(ASSISTANT_HISTORY_STORE_NAME)) {
        db.createObjectStore(ASSISTANT_HISTORY_STORE_NAME, { keyPath: 'storageKey' });
      }
    };

    request.onsuccess = () => {
      resolve(request.result);
    };
  });
}

async function withHistoryStore<T>(
  mode: IDBTransactionMode,
  handler: (store: IDBObjectStore) => Promise<T>,
): Promise<T> {
  const db = await openAssistantHistoryDb();

  return new Promise<T>((resolve, reject) => {
    const transaction = db.transaction(ASSISTANT_HISTORY_STORE_NAME, mode);
    const store = transaction.objectStore(ASSISTANT_HISTORY_STORE_NAME);

    transaction.onabort = () => {
      reject(transaction.error ?? new Error('indexedDB transaction aborted'));
    };
    transaction.onerror = () => {
      reject(transaction.error ?? new Error('indexedDB transaction failed'));
    };

    handler(store)
      .then((result) => {
        transaction.oncomplete = () => {
          db.close();
          resolve(result);
        };
      })
      .catch((error) => {
        db.close();
        reject(error);
      });
  });
}

function readFallbackHistory(storageKey: string, conversationId: string): AssistantHistoryPayload {
  if (!canUseBrowserStorage()) {
    return getEmptyPayload(conversationId);
  }

  const raw = window.localStorage.getItem(buildFallbackStorageKey(storageKey));
  if (!raw) {
    return getEmptyPayload(conversationId);
  }

  try {
    const parsed = JSON.parse(raw) as AssistantHistoryPayload;
    return {
      conversationId,
      updatedAt: typeof parsed.updatedAt === 'string' ? parsed.updatedAt : null,
      messages: Array.isArray(parsed.messages) ? parsed.messages : [],
    };
  } catch {
    return getEmptyPayload(conversationId);
  }
}

function writeFallbackHistory(storageKey: string, payload: AssistantHistoryPayload): AssistantHistoryPayload {
  if (!canUseBrowserStorage()) {
    return payload;
  }

  window.localStorage.setItem(buildFallbackStorageKey(storageKey), JSON.stringify(payload));
  return payload;
}

function deleteFallbackHistory(storageKey: string): void {
  if (!canUseBrowserStorage()) {
    return;
  }

  window.localStorage.removeItem(buildFallbackStorageKey(storageKey));
}

function readStoreValue<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error ?? new Error('indexedDB request failed'));
  });
}

function writeStoreValue<T>(request: IDBRequest<T>): Promise<void> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve();
    request.onerror = () => reject(request.error ?? new Error('indexedDB write failed'));
  });
}

function deleteStoreValue(request: IDBRequest<undefined>): Promise<void> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve();
    request.onerror = () => reject(request.error ?? new Error('indexedDB delete failed'));
  });
}

async function readAssistantHistoryFromIndexedDb(storageKey: string, conversationId: string): Promise<AssistantHistoryPayload> {
  const record = await withHistoryStore('readonly', async (store) => {
    return readStoreValue<AssistantHistoryRecord | undefined>(store.get(storageKey));
  });

  if (!record) {
    return getEmptyPayload(conversationId);
  }

  return {
    conversationId,
    updatedAt: record.updatedAt,
    messages: Array.isArray(record.messages) ? record.messages : [],
  };
}

async function writeAssistantHistoryToIndexedDb(storageKey: string, payload: AssistantHistoryPayload): Promise<AssistantHistoryPayload> {
  const record: AssistantHistoryRecord = {
    storageKey,
    conversationId: payload.conversationId,
    updatedAt: payload.updatedAt,
    messages: payload.messages,
  };

  await withHistoryStore('readwrite', async (store) => {
    await writeStoreValue(store.put(record));
  });

  return payload;
}

async function deleteAssistantHistoryFromIndexedDb(storageKey: string): Promise<void> {
  await withHistoryStore('readwrite', async (store) => {
    await deleteStoreValue(store.delete(storageKey));
  });
}

export async function fetchAssistantHistory(
  userId: string,
  conversationId = DEFAULT_ASSISTANT_CONVERSATION_ID,
): Promise<AssistantHistoryPayload> {
  if (!userId) {
    return getEmptyPayload(conversationId);
  }

  const storageKey = buildStorageKey(userId, conversationId);

  try {
    return await readAssistantHistoryFromIndexedDb(storageKey, conversationId);
  } catch {
    return readFallbackHistory(storageKey, conversationId);
  }
}

export async function saveAssistantHistory(
  userId: string,
  messages: AssistantHistoryMessage[],
  conversationId = DEFAULT_ASSISTANT_CONVERSATION_ID,
): Promise<AssistantHistoryPayload> {
  if (!userId) {
    return getEmptyPayload(conversationId);
  }

  const payload: AssistantHistoryPayload = {
    conversationId,
    updatedAt: new Date().toISOString(),
    messages,
  };
  const storageKey = buildStorageKey(userId, conversationId);

  try {
    return await writeAssistantHistoryToIndexedDb(storageKey, payload);
  } catch {
    return writeFallbackHistory(storageKey, payload);
  }
}

export async function clearAssistantHistory(
  userId: string,
  conversationId = DEFAULT_ASSISTANT_CONVERSATION_ID,
): Promise<void> {
  if (!userId) {
    return;
  }

  const storageKey = buildStorageKey(userId, conversationId);

  try {
    await deleteAssistantHistoryFromIndexedDb(storageKey);
  } catch {
    deleteFallbackHistory(storageKey);
  }
}