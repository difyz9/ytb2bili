'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type SetStateAction } from 'react';

interface UseUserPreferenceSyncOptions<T> {
  userId?: string;
  storageKey: string;
  defaultValue: T;
  isValid?: (value: T) => boolean;
  remoteValue?: T | null;
  remoteLoaded: boolean;
  persistRemote?: (value: T) => Promise<void>;
  isRemoteSynced?: (value: T) => boolean;
  onPersistError?: (error: unknown) => void;
  serialize?: (value: T) => string;
  deserialize?: (raw: string) => T | null;
  valuesEqual?: (left: T, right: T) => boolean;
}

function buildUserPreferenceStorageKey(storageKey: string, userId?: string): string {
  return `${storageKey}:${userId || 'anonymous'}`;
}

function readUserPreference(storageKey: string, userId?: string): string | null {
  if (typeof window === 'undefined') return null;

  try {
    return localStorage.getItem(buildUserPreferenceStorageKey(storageKey, userId));
  } catch {
    return null;
  }
}

function writeUserPreference(storageKey: string, userId: string | undefined, value: string) {
  if (typeof window === 'undefined' || !value) return;

  try {
    localStorage.setItem(buildUserPreferenceStorageKey(storageKey, userId), value);
  } catch {
    // Keep the in-memory preference even if local persistence fails.
  }
}

export function useUserPreferenceSync<T>({
  userId,
  storageKey,
  defaultValue,
  isValid,
  remoteValue,
  remoteLoaded,
  persistRemote,
  isRemoteSynced,
  onPersistError,
  serialize,
  deserialize,
  valuesEqual,
}: UseUserPreferenceSyncOptions<T>) {
  const [value, setInternalValue] = useState<T>(defaultValue);
  const pendingRemoteValueRef = useRef<T | null>(null);
  const requestedPersistTokenRef = useRef<string | null>(null);
  const inFlightPersistTokenRef = useRef<string | null>(null);
  const completedPersistTokenRef = useRef<string | null>(null);

  const checkValid = useMemo(() => isValid ?? (() => true), [isValid]);
  const encode = useMemo(
    () => serialize ?? ((nextValue: T) => JSON.stringify(nextValue)),
    [serialize],
  );
  const decode = useMemo(
    () => deserialize ?? ((raw: string) => {
      try {
        return JSON.parse(raw) as T;
      } catch {
        return null;
      }
    }),
    [deserialize],
  );
  const isEqual = useMemo(() => valuesEqual ?? Object.is, [valuesEqual]);

  useEffect(() => {
    requestedPersistTokenRef.current = null;
    inFlightPersistTokenRef.current = null;
    completedPersistTokenRef.current = null;
  }, [storageKey, userId]);

  const setValue = useCallback((nextValue: SetStateAction<T>) => {
    setInternalValue((previousValue) => {
      const resolvedValue = typeof nextValue === 'function'
        ? (nextValue as (prevState: T) => T)(previousValue)
        : nextValue;

      pendingRemoteValueRef.current = resolvedValue;
      requestedPersistTokenRef.current = encode(resolvedValue);
      inFlightPersistTokenRef.current = null;
      completedPersistTokenRef.current = null;
      return resolvedValue;
    });
  }, [encode]);

  useEffect(() => {
    if (!checkValid(value)) {
      pendingRemoteValueRef.current = null;
      requestedPersistTokenRef.current = null;
      inFlightPersistTokenRef.current = null;
      completedPersistTokenRef.current = null;
      setInternalValue(defaultValue);
    }
  }, [checkValid, defaultValue, value]);

  useEffect(() => {
    pendingRemoteValueRef.current = null;
    requestedPersistTokenRef.current = null;
    inFlightPersistTokenRef.current = null;
    completedPersistTokenRef.current = null;

    const localRaw = readUserPreference(storageKey, userId);
    if (!localRaw) {
      setInternalValue((prevValue) => (isEqual(prevValue, defaultValue) ? prevValue : defaultValue));
      return;
    }

    const localValue = decode(localRaw);
    if (localValue == null || !checkValid(localValue)) {
      setInternalValue((prevValue) => (isEqual(prevValue, defaultValue) ? prevValue : defaultValue));
      return;
    }

    setInternalValue((prevValue) => (isEqual(prevValue, localValue) ? prevValue : localValue));
  }, [checkValid, decode, defaultValue, isEqual, storageKey, userId]);

  useEffect(() => {
    if (!remoteLoaded || remoteValue == null) return;
    if (!checkValid(remoteValue)) return;

    const pendingRemoteValue = pendingRemoteValueRef.current;
    if (pendingRemoteValue != null && checkValid(pendingRemoteValue)) {
      if (isEqual(pendingRemoteValue, remoteValue)) {
        pendingRemoteValueRef.current = null;
        requestedPersistTokenRef.current = null;
        inFlightPersistTokenRef.current = null;
        completedPersistTokenRef.current = null;
      } else {
        return;
      }
    }

    setInternalValue((prevValue) => (isEqual(prevValue, remoteValue) ? prevValue : remoteValue));
    writeUserPreference(storageKey, userId, encode(remoteValue));
  }, [checkValid, encode, isEqual, remoteLoaded, remoteValue, storageKey, userId]);

  useEffect(() => {
    if (!checkValid(value)) return;
    writeUserPreference(storageKey, userId, encode(value));
  }, [checkValid, encode, storageKey, userId, value]);

  useEffect(() => {
    if (!persistRemote || !remoteLoaded || !userId) return;
    if (!checkValid(value)) return;
    const persistToken = encode(value);

    if (requestedPersistTokenRef.current !== persistToken) {
      return;
    }

    if (isRemoteSynced?.(value)) {
      requestedPersistTokenRef.current = null;
      inFlightPersistTokenRef.current = null;
      completedPersistTokenRef.current = persistToken;
      return;
    }

    if (inFlightPersistTokenRef.current === persistToken || completedPersistTokenRef.current === persistToken) {
      return;
    }

    requestedPersistTokenRef.current = null;
    inFlightPersistTokenRef.current = persistToken;

    void persistRemote(value)
      .then(() => {
        if (inFlightPersistTokenRef.current === persistToken) {
          inFlightPersistTokenRef.current = null;
          completedPersistTokenRef.current = persistToken;
        }
      })
      .catch((error) => {
        if (inFlightPersistTokenRef.current === persistToken) {
          inFlightPersistTokenRef.current = null;
        }
        pendingRemoteValueRef.current = null;
        completedPersistTokenRef.current = null;
        onPersistError?.(error);
        // Keep local state even if the remote write fails.
      });
  }, [checkValid, encode, isRemoteSynced, onPersistError, persistRemote, remoteLoaded, userId, value]);

  return {
    value,
    setValue,
  };
}