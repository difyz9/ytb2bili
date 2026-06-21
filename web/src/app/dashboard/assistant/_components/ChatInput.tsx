'use client';

import { useRef, useState } from 'react';
import { Send } from 'lucide-react';
import { useI18n } from '@/contexts/I18nContext';
import { useSlashCommands } from './hooks/useSlashCommands';

interface Props {
  onSend: (text: string) => void;
  disabled?: boolean;
  placeholder?: string;
}

export default function ChatInput({ onSend, disabled, placeholder }: Props) {
  const [input, setInput] = useState('');
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const { t } = useI18n();

  const {
    slashOpen, setSlashOpen,
    slashIndex, setSlashIndex,
    filteredSlash,
    selectCommand,
    handleInputChange,
    slashMenuRef,
  } = useSlashCommands(input, setInput, inputRef);

  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setInput(e.target.value);
    handleInputChange(e.target.value, e.target.selectionStart ?? e.target.value.length);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (slashOpen && filteredSlash.length > 0) {
      if (e.key === 'ArrowDown') { e.preventDefault(); setSlashIndex(i => Math.min(i + 1, filteredSlash.length - 1)); return; }
      if (e.key === 'ArrowUp')   { e.preventDefault(); setSlashIndex(i => Math.max(i - 1, 0)); return; }
      if (e.key === 'Enter' || e.key === 'Tab') { e.preventDefault(); selectCommand(filteredSlash[slashIndex]); return; }
      if (e.key === 'Escape')    { e.preventDefault(); setSlashOpen(false); return; }
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      if (input.trim() && !disabled) {
        onSend(input.trim());
        setInput('');
      }
    }
  };

  const handleSend = () => {
    if (input.trim() && !disabled) {
      onSend(input.trim());
      setInput('');
    }
  };

  const categories = Array.from(new Set(filteredSlash.map(c => c.category)));
  let globalIdx = 0;

  return (
    <div className="p-6 bg-background">
      <div className="relative">
        {slashOpen && filteredSlash.length > 0 && (
          <div
            ref={slashMenuRef}
            className="absolute bottom-full mb-2 left-0 w-full max-h-72 overflow-y-auto z-50 rounded-xl border border-border bg-background shadow-xl [isolation:isolate]"
          >
            {categories.map(cat => (
              <div key={cat}>
                <p className="px-3 pt-2 pb-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground select-none">{cat}</p>
                {filteredSlash
                  .filter(c => c.category === cat)
                  .map(cmd => {
                    const idx = globalIdx++;
                    const selected = idx === slashIndex;
                    return (
                      <button
                        key={cmd.id}
                        data-selected={selected ? 'true' : 'false'}
                        onMouseDown={e => { e.preventDefault(); selectCommand(cmd); }}
                        onMouseEnter={() => setSlashIndex(idx)}
                        className={`w-full flex items-center gap-3 px-3 py-2 text-left transition-colors ${selected ? 'bg-accent' : 'hover:bg-accent/50'}`}
                      >
                        <span className="text-lg w-6 shrink-0 text-center leading-none">{cmd.icon}</span>
                        <div className="min-w-0 flex-1">
                          <p className="text-sm font-medium leading-tight truncate">{cmd.label}</p>
                          <p className="text-xs text-muted-foreground truncate">{cmd.desc}</p>
                        </div>
                      </button>
                    );
                  })}
              </div>
            ))}
            <p className="px-3 py-1.5 text-[10px] text-muted-foreground border-t border-border select-none">
              {t('Use ↑↓ to navigate · Enter / Tab to select · Esc to close')}
            </p>
          </div>
        )}

        <div className="relative rounded-2xl border border-border bg-background focus-within:ring-2 focus-within:ring-primary">
          <textarea
            ref={inputRef}
            value={input}
            onChange={handleChange}
            onKeyDown={handleKeyDown}
            disabled={disabled}
            placeholder={placeholder ?? t('Describe a task or paste a YouTube / Douyin link… Type / for quick commands')}
            rows={1}
            className="w-full resize-none rounded-2xl bg-transparent px-4 py-3 pb-12 pr-14 focus:outline-none min-h-[52px] max-h-32 disabled:opacity-50 disabled:cursor-not-allowed"
            style={{ height: 'auto', overflowY: input.split('\n').length > 3 ? 'auto' : 'hidden' }}
          />
          <div className="absolute bottom-2 left-2">
            <p className="text-xs text-muted-foreground">
              {t('Press Enter to send · Shift + Enter for a new line · / quick commands')}
            </p>
          </div>
          <button
            onClick={handleSend}
            disabled={!input.trim() || disabled}
            className="absolute bottom-2 right-2 p-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            <Send className="h-4 w-4" />
          </button>
        </div>
      </div>
    </div>
  );
}
