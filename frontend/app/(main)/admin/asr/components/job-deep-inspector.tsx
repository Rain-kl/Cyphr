// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import type { JobDTO } from '@/lib/services/transcribe';
import {
  AlertCircle,
  Check,
  Code,
  Copy,
  Download,
  FolderOpen,
  Info,
  Server,
  User,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

interface JobDeepInspectorProps {
  job: JobDTO | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function JobDeepInspector({
  job,
  open,
  onOpenChange,
}: JobDeepInspectorProps) {
  const t = useTranslations('adminAsr.allJobs');
  const [copiedJson, setCopiedJson] = React.useState(false);

  if (!job) return null;

  const handleCopyJson = async () => {
    if (!job.result_json) return;
    await navigator.clipboard.writeText(job.result_json);
    setCopiedJson(true);
    toast.success('JSON copied to clipboard');
    setTimeout(() => setCopiedJson(false), 2000);
  };

  const formatTimestamp = (ts?: string) => {
    if (!ts) return '-';
    try {
      return new Date(ts).toLocaleString();
    } catch {
      return ts;
    }
  };

  const formatDuration = (seconds?: number) => {
    if (!seconds) return '-';
    const m = Math.floor(seconds / 60);
    const s = Math.floor(seconds % 60);
    return `${m}m ${s}s (${seconds.toFixed(2)}s)`;
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full sm:max-w-xl overflow-y-auto'>
        <SheetHeader>
          <div className='flex items-center gap-2'>
            <Info className='size-5 text-primary' />
            <SheetTitle className='text-lg font-semibold'>
              Job #{job.id} Inspector
            </SheetTitle>
          </div>
          <SheetDescription>{job.original_file_name}</SheetDescription>
        </SheetHeader>

        <div className='mt-6 space-y-6 text-sm'>
          {/* Metadata Grid */}
          <div className='rounded-xl border bg-muted/20 p-4 space-y-2.5'>
            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground flex items-center gap-1.5'>
                <User className='size-3.5' />
                <span>{t('userCol')}</span>
              </span>
              <span className='font-mono font-semibold'>
                {job.user_id
                  ? `User #${job.user_id}`
                  : 'Anonymous / Direct API'}
              </span>
            </div>

            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground flex items-center gap-1.5'>
                <Server className='size-3.5' />
                <span>{t('nodeCol')}</span>
              </span>
              <span className='font-mono font-semibold'>
                {job.node_id ? `Node #${job.node_id}` : 'Unassigned / Pending'}
              </span>
            </div>

            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground'>Status</span>
              <Badge variant='outline' className='font-mono uppercase'>
                {job.status} ({job.progress}%)
              </Badge>
            </div>

            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground'>Model</span>
              <code className='rounded bg-muted px-1.5 py-0.5 text-xs font-mono'>
                {job.model} ({job.task_type})
              </code>
            </div>

            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground'>Duration</span>
              <span className='font-mono text-xs text-muted-foreground'>
                {formatDuration(job.duration)}
              </span>
            </div>

            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground'>Created At</span>
              <span className='text-xs text-muted-foreground'>
                {formatTimestamp(job.created_at)}
              </span>
            </div>

            {job.started_at && (
              <div className='flex items-center justify-between'>
                <span className='text-xs text-muted-foreground'>
                  Started At
                </span>
                <span className='text-xs text-muted-foreground'>
                  {formatTimestamp(job.started_at)}
                </span>
              </div>
            )}

            {job.completed_at && (
              <div className='flex items-center justify-between'>
                <span className='text-xs text-muted-foreground'>
                  Completed At
                </span>
                <span className='text-xs text-muted-foreground'>
                  {formatTimestamp(job.completed_at)}
                </span>
              </div>
            )}
          </div>

          {/* Physical Storage Path and Download */}
          <div className='space-y-1.5'>
            <span className='text-xs font-medium text-muted-foreground flex items-center gap-1.5'>
              <FolderOpen className='size-3.5 text-primary' />
              <span>{t('storagePath')}</span>
            </span>
            <div className='flex items-center justify-between gap-2 rounded-xl border bg-muted/30 p-3'>
              <span className='font-mono text-xs text-muted-foreground break-all'>
                {job.audio_storage_path || 'Direct stream storage'}
              </span>
              <a
                href={`/api/v1/agent/jobs/${job.id}/media`}
                target='_blank'
                rel='noreferrer'
                download
              >
                <Button
                  variant='outline'
                  size='sm'
                  className='gap-1 text-xs shrink-0'
                >
                  <Download className='size-3.5' />
                  <span>Download</span>
                </Button>
              </a>
            </div>
          </div>

          {/* System Error if Failed */}
          {job.status === 'failed' && (
            <div className='space-y-1.5'>
              <span className='text-xs font-semibold text-rose-600 dark:text-rose-400 flex items-center gap-1.5'>
                <AlertCircle className='size-3.5' />
                <span>{t('rawError')}</span>
              </span>
              <div className='rounded-xl border border-rose-500/20 bg-rose-500/10 p-3 font-mono text-xs text-rose-600 dark:text-rose-400 select-text break-all'>
                {job.error_msg || 'No detailed error reported.'}
              </div>
            </div>
          )}

          {/* Raw OpenAI JSON Viewer */}
          <div className='space-y-1.5'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground flex items-center gap-1.5'>
                <Code className='size-3.5 text-primary' />
                <span>{t('rawJson')}</span>
              </span>
              {job.result_json && (
                <Button
                  variant='ghost'
                  size='sm'
                  onClick={handleCopyJson}
                  className='h-7 gap-1 text-xs'
                >
                  {copiedJson ? (
                    <Check className='size-3 text-emerald-500' />
                  ) : (
                    <Copy className='size-3' />
                  )}
                  <span>{t('copyJson')}</span>
                </Button>
              )}
            </div>

            <ScrollArea className='h-60 rounded-xl border bg-zinc-950 p-3 font-mono text-[11px] text-zinc-200 select-text dark:bg-zinc-900'>
              {job.result_json ? (
                <pre className='whitespace-pre-wrap break-all leading-relaxed'>
                  {(() => {
                    try {
                      return JSON.stringify(
                        JSON.parse(job.result_json),
                        null,
                        2,
                      );
                    } catch {
                      return job.result_json;
                    }
                  })()}
                </pre>
              ) : (
                <div className='flex h-full items-center justify-center text-zinc-500 italic'>
                  No raw JSON response recorded yet.
                </div>
              )}
            </ScrollArea>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}
