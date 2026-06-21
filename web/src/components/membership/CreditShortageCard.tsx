'use client';

import { cn } from '@/lib/utils';

export interface CreditShortageCardProps {
  availableCredits: number;
  requiredCredits: number;
  serviceLabel: string;
  recommendedPlanId: string;
  className?: string;
}

/**
 * Stub component — the credits / payment system has been removed.
 * Renders a minimal notice so existing UI layouts don't break.
 */
export default function CreditShortageCard({
  availableCredits,
  requiredCredits,
  serviceLabel,
  className,
}: CreditShortageCardProps) {
  const shortage = Math.max(requiredCredits - availableCredits, 0);

  return (
    <div className={cn('rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800', className)}>
      <p className="font-semibold">Insufficient credits</p>
      <p className="mt-1">
        {serviceLabel} requires {requiredCredits} credits. You have {availableCredits} available
        (short by {shortage}).
      </p>
      <p className="mt-2 text-xs text-amber-600">
        The credits system has been disabled. Please contact the administrator.
      </p>
    </div>
  );
}
