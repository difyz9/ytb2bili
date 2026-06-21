import { cn } from '@/lib/utils';
import { translateClientText } from '@/lib/i18n';

interface StatusBadgeProps {
  status: string;
  text?: string;
  className?: string;
}

const statusConfig: Record<string, { label: string; color: string }> = {
  pending: { label: 'Pending', color: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200' },
  preparing: { label: 'Preparing', color: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200' },
  ready: { label: 'Ready', color: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' },
  uploading: { label: 'Uploading', color: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200' },
  completed: { label: 'Completed', color: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' },
  failed: { label: 'Failed', color: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200' },
  cancelled: { label: 'Cancelled', color: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200' },
  enabled: { label: 'Enabled', color: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' },
  disabled: { label: 'Disabled', color: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200' },
  error: { label: 'Error', color: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200' },
};

export function StatusBadge({ status, text, className }: StatusBadgeProps) {
  const config = statusConfig[status] || { label: status, color: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200' };
  const displayText = translateClientText(text || config.label);

  return (
    <span
      className={cn(
        'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
        config.color,
        className
      )}
    >
      {displayText}
    </span>
  );
}
