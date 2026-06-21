'use client';

import { useState } from 'react';
import Link from 'next/link';
import { X } from 'lucide-react';

export interface UpgradePromptCardProps {
  badge: string;
  title: string;
  description: string;
  highlights: string[];
  primaryHref: string;
  primaryLabel: string;
  secondaryHref: string;
  secondaryLabel: string;
  dismissible?: boolean;
  onDismiss?: () => void;
}

/**
 * Stub component — the membership upgrade / payment system has been removed.
 * Renders a minimal upsell notice so existing UI layouts don't break.
 */
export default function UpgradePromptCard({
  badge,
  title,
  description,
  highlights,
  primaryLabel,
  secondaryLabel,
  dismissible,
  onDismiss,
}: UpgradePromptCardProps) {
  const [dismissed, setDismissed] = useState(false);

  if (dismissible && dismissed) {
    return null;
  }

  return (
    <div className="relative rounded-xl border border-blue-200 bg-blue-50 p-4">
      {dismissible && onDismiss ? (
        <button
          type="button"
          onClick={() => {
            setDismissed(true);
            onDismiss();
          }}
          className="absolute right-3 top-3 rounded-full p-1 text-blue-400 hover:bg-blue-100 hover:text-blue-600"
          aria-label="Dismiss"
        >
          <X className="h-4 w-4" />
        </button>
      ) : null}

      <span className="inline-block rounded-full bg-blue-100 px-2.5 py-0.5 text-xs font-semibold text-blue-700">
        {badge}
      </span>

      <h3 className="mt-3 text-sm font-semibold text-blue-900">{title}</h3>
      <p className="mt-1 text-xs text-blue-700">{description}</p>

      {highlights.length > 0 ? (
        <ul className="mt-3 space-y-1">
          {highlights.map((item) => (
            <li key={item} className="flex items-center gap-2 text-xs text-blue-700">
              <span className="h-1.5 w-1.5 rounded-full bg-blue-400" />
              {item}
            </li>
          ))}
        </ul>
      ) : null}

      <div className="mt-4 flex items-center gap-3">
        <span className="text-xs font-medium text-blue-600">{primaryLabel}</span>
        <Link
          href="/dashboard"
          className="text-xs font-medium text-blue-500 hover:underline"
        >
          {secondaryLabel}
        </Link>
      </div>
    </div>
  );
}
