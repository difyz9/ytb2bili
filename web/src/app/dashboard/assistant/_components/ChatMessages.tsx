'use client';

import { useEffect, useRef } from 'react';
import { Trash2 } from 'lucide-react';
import { useI18n } from '@/contexts/I18nContext';
import AssistantBubble, { type Message } from './AssistantBubble';

interface Props {
  messages: Message[];
  isLoading: boolean;
  onSuggestedPrompt: (prompt: string) => void;
  disabled?: boolean;
  onTrackingStatusChange?: (messageId: string, payload: { status: 'tracking' | 'completed' | 'failed'; biliBvid?: string }) => void;
  onDeleteMessage?: (messageId: string) => void;
}

export default function ChatMessages({ messages, isLoading, onSuggestedPrompt, disabled, onTrackingStatusChange, onDeleteMessage }: Props) {
  const endRef = useRef<HTMLDivElement>(null);
  const { locale, t } = useI18n();

  const suggestedPrompts = [
    t('Submit this video to the full pipeline: https://www.youtube.com/watch?v=dQw4w9WgXcQ'),
    t('Show the most recently failed videos'),
    t('Rewrite the Bilibili title for video dQw4w9WgXcQ and emphasize that it is a music video'),
    t('Summarize the content of video dQw4w9WgXcQ'),
    t('List all channels I am subscribed to'),
  ];

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  return (
    <div className="flex-1 overflow-y-auto p-6 space-y-6">
      {messages.map(message => (
        <div key={message.id} className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}>
          {message.role === 'user' ? (
            <div className="group relative max-w-[80%] rounded-2xl px-4 py-3 bg-primary text-primary-foreground">
              {onDeleteMessage ? (
                <button
                  type="button"
                  onClick={() => onDeleteMessage(message.id)}
                  className="absolute right-3 top-3 rounded-md p-1 text-primary-foreground/80 opacity-0 transition hover:bg-white/10 hover:text-white group-hover:opacity-100"
                  title={t('Delete this history message')}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              ) : null}
              <p className="whitespace-pre-wrap">{message.content}</p>
              <p className="text-xs mt-1 text-primary-foreground/70">
                {message.timestamp.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' })}
              </p>
            </div>
          ) : (
            <AssistantBubble message={message} onTrackingStatusChange={onTrackingStatusChange} onDelete={onDeleteMessage} />
          )}
        </div>
      ))}

      {messages.length === 1 && (
        <div className="space-y-3">
          <p className="text-sm text-muted-foreground text-center">{t('Try these common actions:')}</p>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {suggestedPrompts.map((prompt, i) => (
              <button
                key={i}
                onClick={() => !disabled && onSuggestedPrompt(prompt)}
                disabled={disabled}
                className="p-4 text-left rounded-xl border border-border hover:border-primary hover:bg-accent transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <p className="text-sm">{prompt}</p>
              </button>
            ))}
          </div>
        </div>
      )}

      {isLoading && (
        <div className="flex justify-start">
          <div className="bg-muted rounded-2xl px-4 py-3">
            <div className="flex items-center space-x-2">
              <div className="w-2 h-2 bg-foreground/50 rounded-full animate-bounce" />
              <div className="w-2 h-2 bg-foreground/50 rounded-full animate-bounce [animation-delay:150ms]" />
              <div className="w-2 h-2 bg-foreground/50 rounded-full animate-bounce [animation-delay:300ms]" />
            </div>
          </div>
        </div>
      )}

      <div ref={endRef} />
    </div>
  );
}
