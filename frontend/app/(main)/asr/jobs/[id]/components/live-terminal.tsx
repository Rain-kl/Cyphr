// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import type { LogMessage } from '@/lib/services/transcribe';
import {
  ChevronDown,
  ChevronUp,
  Maximize2,
  Minimize2,
  Terminal as TerminalIcon,
  Trash2,
} from 'lucide-react';
import { useTranslations } from 'next-intl';

interface LiveTerminalProps {
  logs: LogMessage[];
  isRunning: boolean;
  onClear?: () => void;
}

export function LiveTerminal({ logs, isRunning, onClear }: LiveTerminalProps) {
  const t = useTranslations('asr.jobDetail');

  const [autoScroll, setAutoScroll] = React.useState(true);
  const [isExpanded, setIsExpanded] = React.useState(true);
  const [isFullscreen, setIsFullscreen] = React.useState(false);

  const containerRef = React.useRef<HTMLDivElement | null>(null);

  React.useEffect(() => {
    if (autoScroll && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [logs, autoScroll]);

  const formatTimestamp = (ts: string) => {
    try {
      const d = new Date(ts);
      return d.toLocaleTimeString(undefined, {
        hour12: false,
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      });
    } catch {
      return ts;
    }
  };

  return (
    <div
      className={`rounded-xl border border-zinc-800 bg-zinc-950 text-zinc-200 shadow-md transition-all ${
        isFullscreen
          ? 'fixed inset-4 z-50 flex flex-col'
          : 'flex flex-col overflow-hidden'
      }`}
    >
      {/* Terminal Title Bar */}
      <div className='flex items-center justify-between border-b border-zinc-800/80 px-4 py-2.5 bg-zinc-900/50'>
        <div className='flex items-center gap-2'>
          <TerminalIcon className='size-4 text-emerald-400' />
          <span className='font-mono text-xs font-semibold tracking-wide'>
            {t('liveLogTitle')}
          </span>
          <Badge
            variant='outline'
            className='border-zinc-700 bg-zinc-800/50 px-1.5 py-0 text-[10px] font-mono text-zinc-400'
          >
            {logs.length} lines
          </Badge>
          {isRunning && (
            <span className='flex h-2 w-2 relative'>
              <span className='animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75'></span>
              <span className='relative inline-flex rounded-full h-2 w-2 bg-emerald-500'></span>
            </span>
          )}
        </div>

        <div className='flex items-center gap-3'>
          {/* Auto Scroll Switch */}
          <div className='flex items-center gap-1.5'>
            <span className='text-[11px] font-mono text-zinc-400'>
              {t('autoScroll')}
            </span>
            <Switch
              checked={autoScroll}
              onCheckedChange={setAutoScroll}
              className='scale-75'
              aria-label={t('autoScroll')}
            />
          </div>

          {/* Clear Button */}
          {onClear && (
            <Button
              variant='ghost'
              size='icon'
              className='size-7 text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800'
              onClick={onClear}
              aria-label={t('clearLogs')}
            >
              <Trash2 className='size-3.5' />
            </Button>
          )}

          {/* Fullscreen Toggle */}
          <Button
            variant='ghost'
            size='icon'
            className='size-7 text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800'
            onClick={() => setIsFullscreen(!isFullscreen)}
            aria-label={t('fullscreen')}
          >
            {isFullscreen ? (
              <Minimize2 className='size-3.5' />
            ) : (
              <Maximize2 className='size-3.5' />
            )}
          </Button>

          {/* Collapse Toggle */}
          {!isFullscreen && (
            <Button
              variant='ghost'
              size='icon'
              className='size-7 text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800'
              onClick={() => setIsExpanded(!isExpanded)}
              aria-label={isExpanded ? 'Collapse' : 'Expand'}
            >
              {isExpanded ? (
                <ChevronUp className='size-3.5' />
              ) : (
                <ChevronDown className='size-3.5' />
              )}
            </Button>
          )}
        </div>
      </div>

      {/* Terminal Log Console */}
      {isExpanded && (
        <div
          ref={containerRef}
          className={`overflow-y-auto p-4 font-mono text-[12px] leading-relaxed select-text ${
            isFullscreen ? 'flex-1' : 'h-64'
          }`}
        >
          {logs.length === 0 ? (
            <div className='flex h-full items-center justify-center text-zinc-500 italic'>
              Waiting for execution logs from agent...
            </div>
          ) : (
            logs.map((log, index) => (
              <div
                key={`${log.timestamp}-${index}`}
                className='flex items-start gap-2 hover:bg-zinc-900/40 py-0.5 px-1 rounded'
              >
                <span className='text-zinc-500 shrink-0 select-none'>
                  [{formatTimestamp(log.timestamp)}]
                </span>
                <span className='text-emerald-400 font-semibold shrink-0 select-none'>
                  {log.progress > 0 ? `[${log.progress}%]` : '[-]'}
                </span>
                <span className='text-zinc-200 whitespace-pre-wrap break-all'>
                  {log.message}
                </span>
              </div>
            ))
          )}
        </div>
      )}
    </div>
  );
}
