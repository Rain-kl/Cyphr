// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import type {
  JobDTO,
  TranscriptSegment,
  VerboseJSONResult,
} from '@/lib/services/transcribe';
import {
  Check,
  Code2,
  Copy,
  FileText,
  Hash,
  Play,
  Volume2,
} from 'lucide-react';
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

  const fullText = job.result_text || parsedVerbose?.text || '';

  // Fallback: If no structured segments exist but fullText does, create a single root AST segment
  const segments: TranscriptSegment[] = React.useMemo(() => {
    const raw = parsedVerbose?.segments || [];
    if (raw.length > 0) return raw;
    if (fullText) {
      return [
        {
          id: 0,
          start: 0,
          end: job.duration || 0,
          text: fullText,
        },
      ];
    }
    return [];
  }, [parsedVerbose, fullText, job.duration]);

  // Determine which segment is currently active
  const activeSegmentId = React.useMemo(() => {
    if (segments.length === 0) return null;
    const found = segments.find(
      (seg) => currentTime >= seg.start && currentTime < seg.end,
    );
    if (found) return found.id;
    // Edge case: if right at the end of the last segment
    const last = segments[segments.length - 1];
    if (last && currentTime >= last.start && currentTime <= last.end + 0.5) {
      return last.id;
    }
    return null;
  }, [segments, currentTime]);

  // Smoothly auto-scroll to the active segment when it changes
  React.useEffect(() => {
    if (activeSegmentId !== null && activeSegmentRef.current) {
      activeSegmentRef.current.scrollIntoView({
        behavior: 'smooth',
        block: 'nearest',
      });
    }
  }, [activeSegmentId]);

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
    <div className='border border-dashed shadow-none rounded-lg overflow-hidden bg-background'>
      <Tabs defaultValue='ast' className='w-full'>
        {/* Header Bar */}
        <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between border-b border-dashed px-4 py-2.5 bg-muted/20 gap-3'>
          <div className='flex items-center gap-3'>
            <div className='flex items-center gap-2'>
              <FileText className='size-4 text-primary' />
              <h2 className='text-sm font-semibold tracking-tight text-foreground'>
                {t('transcriptTitle')}
              </h2>
            </div>

            {/* Tab switch */}
            <TabsList className='h-8 p-0.5 border border-dashed shadow-none bg-background'>
              <TabsTrigger
                value='ast'
                className='gap-1.5 text-xs h-7 px-2.5 data-[state=active]:bg-muted data-[state=active]:shadow-none'
              >
                <Code2 className='size-3.5' />
                <span>{t('tabAst')}</span>
              </TabsTrigger>
              <TabsTrigger
                value='text'
                className='gap-1.5 text-xs h-7 px-2.5 data-[state=active]:bg-muted data-[state=active]:shadow-none'
              >
                <FileText className='size-3.5' />
                <span>{t('tabPlainText')}</span>
              </TabsTrigger>
            </TabsList>
          </div>

          {/* Action and stats */}
          <div className='flex items-center gap-2'>
            {fullText && (
              <span className='text-[11px] font-mono text-muted-foreground hidden md:inline-block'>
                {fullText.length} chars
              </span>
            )}
            <Button
              variant='outline'
              size='sm'
              disabled={!fullText}
              onClick={handleCopy}
              className='h-7 gap-1.5 px-2.5 text-xs border-dashed shadow-none'
            >
              {copied ? (
                <Check className='size-3 text-emerald-500' />
              ) : (
                <Copy className='size-3 text-muted-foreground' />
              )}
              <span>{t('copyText')}</span>
            </Button>
          </div>
        </div>

        {/* 1. AST View Mode */}
        <TabsContent value='ast' className='m-0 p-0 focus-visible:outline-none'>
          {segments.length === 0 ? (
            <div className='flex h-48 items-center justify-center text-xs text-muted-foreground'>
              {t('noTranscript')}
            </div>
          ) : (
            <ScrollArea className='h-[460px] p-4'>
              <div className='space-y-2.5'>
                {segments.map((seg, idx) => {
                  const isActive = seg.id === activeSegmentId;

                  return (
                    <div
                      key={seg.id ?? idx}
                      ref={isActive ? activeSegmentRef : null}
                      onClick={() => onSeek(seg.start)}
                      title={t('clickToSeek')}
                      className={`group flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-all ${
                        isActive
                          ? 'border-primary/50 bg-primary/10 shadow-xs ring-1 ring-primary/20'
                          : 'border-dashed border-border/70 bg-muted/5 hover:border-primary/40 hover:bg-muted/15'
                      }`}
                    >
                      {/* Left: Time and Status Button */}
                      <button
                        type='button'
                        onClick={(e) => {
                          e.stopPropagation();
                          onSeek(seg.start);
                        }}
                        className={`flex shrink-0 items-center gap-1.5 rounded-full px-2 py-0.5 font-mono text-[10px] font-medium transition-colors ${
                          isActive
                            ? 'bg-primary text-primary-foreground'
                            : 'bg-muted text-muted-foreground group-hover:bg-primary/15 group-hover:text-primary'
                        }`}
                      >
                        {isActive ? (
                          <Volume2 className='size-2.5 animate-pulse' />
                        ) : (
                          <Play className='size-2.5' />
                        )}
                        <span>
                          {formatSeconds(seg.start)} - {formatSeconds(seg.end)}
                        </span>
                      </button>

                      {/* Middle: AST Node Content */}
                      <div className='flex-1 min-w-0 space-y-1.5'>
                        <p
                          className={`text-sm leading-relaxed transition-colors select-text ${
                            isActive
                              ? 'font-medium text-foreground'
                              : 'text-foreground/80 group-hover:text-foreground'
                          }`}
                        >
                          {seg.text.trim()}
                        </p>

                        {/* AST Node Metadata Pills */}
                        <div className='flex flex-wrap items-center gap-2 text-[10px] font-mono text-muted-foreground'>
                          <span className='flex items-center gap-0.5'>
                            <Hash className='size-2.5' />
                            <span>Node #{seg.id ?? idx + 1}</span>
                          </span>
                          <span>•</span>
                          <span>{(seg.end - seg.start).toFixed(1)}s</span>
                          {typeof seg.avg_logprob === 'number' && (
                            <>
                              <span>•</span>
                              <span>logprob: {seg.avg_logprob.toFixed(2)}</span>
                            </>
                          )}
                        </div>
                      </div>

                      {/* Right Indicator */}
                      {isActive && (
                        <Badge
                          variant='outline'
                          className='text-[10px] bg-primary/15 border-primary/30 text-primary rounded-full py-0 px-2 font-medium shrink-0 self-center'
                        >
                          <span className='size-1 bg-primary rounded-full mr-1.5 shrink-0 animate-pulse' />
                          Playing
                        </Badge>
                      )}
                    </div>
                  );
                })}
              </div>
            </ScrollArea>
          )}
        </TabsContent>

        {/* 2. Plain Text View Mode */}
        <TabsContent
          value='text'
          className='m-0 p-0 focus-visible:outline-none'
        >
          <ScrollArea className='h-[460px] p-6'>
            {fullText ? (
              <div className='max-w-4xl space-y-4'>
                <p className='whitespace-pre-wrap text-sm leading-relaxed text-foreground/90 select-text font-sans'>
                  {fullText}
                </p>
              </div>
            ) : (
              <div className='flex h-48 items-center justify-center text-xs text-muted-foreground'>
                {t('noTranscript')}
              </div>
            )}
          </ScrollArea>
        </TabsContent>
      </Tabs>
    </div>
  );
}
