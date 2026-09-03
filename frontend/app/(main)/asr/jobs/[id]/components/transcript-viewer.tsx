// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import type { JobDTO, VerboseJSONResult } from '@/lib/services/transcribe';
import { Check, Copy, FileText, ListOrdered, Play } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

interface TranscriptViewerProps {
  job: JobDTO;
  currentTime: number;
  onSeek: (seconds: number) => void;
}

export function TranscriptViewer({
  job,
  currentTime,
  onSeek,
}: TranscriptViewerProps) {
  const t = useTranslations('asr.jobDetail');
  const [copied, setCopied] = React.useState(false);

  const activeSegmentRef = React.useRef<HTMLDivElement | null>(null);

  const parsedVerbose: VerboseJSONResult | null = React.useMemo(() => {
    if (!job.result_json) return null;
    try {
      return JSON.parse(job.result_json) as VerboseJSONResult;
    } catch {
      return null;
    }
  }, [job.result_json]);

  const segments = parsedVerbose?.segments || [];
  const fullText = job.result_text || parsedVerbose?.text || '';

  const formatSeconds = (sec: number) => {
    const m = Math.floor(sec / 60);
    const s = Math.floor(sec % 60);
    const ms = Math.floor((sec % 1) * 10);
    return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}.${ms}`;
  };

  const handleCopy = async () => {
    if (!fullText) return;
    await navigator.clipboard.writeText(fullText);
    setCopied(true);
    toast.success(t('copied'));
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className='rounded-xl border bg-card shadow-sm overflow-hidden'>
      <Tabs defaultValue={segments.length > 0 ? 'segments' : 'fulltext'}>
        <div className='flex items-center justify-between border-b px-4 py-2.5'>
          <TabsList className='h-8'>
            {segments.length > 0 && (
              <TabsTrigger value='segments' className='gap-1.5 text-xs'>
                <ListOrdered className='size-3.5' />
                <span>{t('tabSegments')}</span>
              </TabsTrigger>
            )}
            <TabsTrigger value='fulltext' className='gap-1.5 text-xs'>
              <FileText className='size-3.5' />
              <span>{t('tabFullText')}</span>
            </TabsTrigger>
          </TabsList>

          <Button
            variant='ghost'
            size='sm'
            disabled={!fullText}
            onClick={handleCopy}
            className='h-8 gap-1.5 px-2 text-xs'
          >
            {copied ? (
              <Check className='size-3.5 text-emerald-500' />
            ) : (
              <Copy className='size-3.5' />
            )}
            <span>{t('copyText')}</span>
          </Button>
        </div>

        {/* Timeline Segments View */}
        {segments.length > 0 && (
          <TabsContent value='segments' className='m-0 p-0'>
            <ScrollArea className='h-[420px] p-4'>
              <div className='space-y-2.5'>
                {segments.map((seg) => {
                  const isActive =
                    currentTime >= seg.start && currentTime <= seg.end;

                  return (
                    <div
                      key={seg.id}
                      ref={isActive ? activeSegmentRef : null}
                      onClick={() => onSeek(seg.start)}
                      className={`group flex cursor-pointer items-start gap-3 rounded-xl border p-3 transition-all ${
                        isActive
                          ? 'border-primary/40 bg-primary/5 shadow-xs'
                          : 'border-transparent bg-muted/20 hover:border-border hover:bg-muted/40'
                      }`}
                    >
                      <button
                        type='button'
                        className={`flex shrink-0 items-center gap-1 rounded-md px-2 py-1 font-mono text-[11px] font-medium transition-colors ${
                          isActive
                            ? 'bg-primary text-primary-foreground'
                            : 'bg-muted text-muted-foreground group-hover:bg-primary/20 group-hover:text-primary'
                        }`}
                      >
                        <Play className='size-2.5' />
                        <span>{formatSeconds(seg.start)}</span>
                      </button>

                      <div className='flex-1 text-sm leading-relaxed'>
                        <p
                          className={
                            isActive
                              ? 'font-medium text-foreground'
                              : 'text-foreground/80'
                          }
                        >
                          {seg.text.trim()}
                        </p>
                      </div>
                    </div>
                  );
                })}
              </div>
            </ScrollArea>
          </TabsContent>
        )}

        {/* Full Text View */}
        <TabsContent value='fulltext' className='m-0 p-0'>
          <ScrollArea className='h-[420px] p-6'>
            {fullText ? (
              <p className='whitespace-pre-wrap text-sm leading-relaxed text-foreground/90 select-text'>
                {fullText}
              </p>
            ) : (
              <div className='flex h-36 items-center justify-center text-xs text-muted-foreground'>
                No transcript text available yet.
              </div>
            )}
          </ScrollArea>
        </TabsContent>
      </Tabs>
    </div>
  );
}
