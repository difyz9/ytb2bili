import { cn } from '@/lib/utils';
import { ReactNode } from 'react';
import { X } from 'lucide-react';

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  children: ReactNode;
  title?: string;
  description?: string;
  className?: string;
}

export function Modal({ isOpen, onClose, children, title, description, className }: ModalProps) {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50 dark:bg-black/70"
        onClick={onClose}
      />

      {/* Modal */}
      <div className={cn(
        'relative bg-white dark:bg-gray-800 rounded-lg shadow-lg max-w-lg w-full max-h-[90vh] overflow-y-auto',
        className
      )}>
        {/* Header */}
        {(title || description) && (
          <div className="p-6 border-b border-border dark:border-gray-700">
            <div className="flex items-start justify-between">
              <div>
                {title && <h2 className="text-xl font-semibold text-gray-900 dark:text-white">{title}</h2>}
                {description && (
                  <p className="text-sm text-muted-foreground mt-1">{description}</p>
                )}
              </div>
              <button
                onClick={onClose}
                className="p-1 hover:bg-accent rounded-lg transition-colors text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
              >
                <X className="h-5 w-5" />
              </button>
            </div>
          </div>
        )}

        {/* Content */}
        {!title && !description ? children : (
          <div className="p-6">
            {children}
          </div>
        )}
      </div>
    </div>
  );
}
