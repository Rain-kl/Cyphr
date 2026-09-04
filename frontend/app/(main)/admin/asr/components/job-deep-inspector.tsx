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
import {
  AdminTranscribeService,
  type JobDTO,
  type JobSummaryDTO,
} from '@/lib/services/transcribe';
import {
  AlertCircle,
  Check,
  Code,
  Copy,
  Download,
  FolderOpen,
  Info,
  Loader2,
  Server,
  User,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

interface JobDeepInspectorProps {
  job: JobSummaryDTO | JobDTO | null;
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
  const [detailJob, setDetailJob] = React.useState<JobDTO | null>(null);
  const [isLoadingDetail, setIsLoadingDetail] = React.useState(false);

  const jobId = job?.id;

  React.useEffect(() => {
    if (!open || !jobId) {
      setDetailJob(null);
      return;
    }
    let isCancelled = false;
    setIsLoadingDetail(true);
    AdminTranscribeService.getJobDetail(jobId)
      .then((res) => {
        if (!isCancelled) {
          setDetailJob(res);
        }
      })
      .catch((err: unknown) => {
        if (!isCancelled) {
          console.error('Failed to load job detail:', err);
        }
      })
      .finally(() => {
        if (!isCancelled) {
          setIsLoadingDetail(false);
        }
      });

    return () => {
      isCancelled = true;
    };
  }, [open, jobId]);

  if (!job) return null;

  const currentJob: JobDTO | JobSummaryDTO = detailJob || job;
  const resultJson = detailJob?.result_json;
  const errorMsg = detailJob?.error_msg;
  const storagePath = detailJob?.audio_storage_path;

  const handleCopyJson = async () => {
    if (!resultJson) return;
    await navigator.clipboard.writeText(resultJson);
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

  const resolveMediaUrl = () => {
    if (detailJob?.media_url) return detailJob.media_url;
    if (storagePath) {
      const match = storagePath.match(/(\d+)\.[^.]+$/);
      if (match?.[1]) {
        return `/f/${match[1]}`;
      }
    }
    return null;
  };
  const mediaUrl = resolveMediaUrl();

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full sm:max-w-2xl overflow-y-auto p-6 space-y-6'>
        <SheetHeader>
          <div className='flex items-center gap-2'>
            <Info className='size-5 text-primary' />
            <SheetTitle className='text-lg font-semibold'>
              Job #{currentJob.id} Inspector
            </SheetTitle>
          </div>
          <SheetDescription>{currentJob.original_file_name}</SheetDescription>
        </SheetHeader>

        <div className='space-y-6 text-sm'>
          {/* Metadata Grid */}
          <div className='rounded-xl border bg-muted/20 p-4 space-y-2.5'>
            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground flex items-center gap-1.5'>
                <User className='size-3.5' />
                <span>{t('userCol')}</span>
              </span>
              <span className='font-mono font-semibold'>
                {currentJob.user_id
                  ? `User #${currentJob.user_id}`
                  : 'Anonymous / Direct API'}
              </span>
            </div>

            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground flex items-center gap-1.5'>
                <Server className='size-3.5' />
                <span>{t('nodeCol')}</span>
              </span>
              <span className='font-mono font-semibold'>
                {currentJob.node_id
                  ? `Node #${currentJob.node_id}`
                  : 'Unassigned / Pending'}
              </span>
            </div>

            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground'>Status</span>
              <div className='flex items-center gap-1.5'>
                <Badge variant='outline' className='font-mono uppercase'>
                  {currentJob.status} ({currentJob.progress}%)
                </Badge>
                {typeof currentJob.retry_count === 'number' &&
                  currentJob.retry_count > 0 && (
                    <Badge variant='secondary' className='text-[10px]'>
                      Retry #{currentJob.retry_count}
                    </Badge>
                  )}
              </div>
            </div>

            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground'>Model</span>
              <code className='rounded bg-muted px-1.5 py-0.5 text-xs font-mono'>
                {currentJob.model} ({currentJob.task_type})
              </code>
            </div>

            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground'>Duration</span>
              <span className='font-mono text-xs text-muted-foreground'>
                {formatDuration(currentJob.duration)}
              </span>
            </div>

            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground'>Created At</span>
              <span className='text-xs text-muted-foreground'>
                {formatTimestamp(currentJob.created_at)}
              </span>
            </div>

            {currentJob.started_at && (
              <div className='flex items-center justify-between'>
                <span className='text-xs text-muted-foreground'>
                  Started At
                </span>
                <span className='text-xs text-muted-foreground'>
                  {formatTimestamp(currentJob.started_at)}
                </span>
              </div>
            )}

            {currentJob.completed_at && (
              <div className='flex items-center justify-between'>
                <span className='text-xs text-muted-foreground'>
                  Completed At
                </span>
                <span className='text-xs text-muted-foreground'>
                  {formatTimestamp(currentJob.completed_at)}
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
              {isLoadingDetail ? (
                <div className='flex items-center gap-2 text-xs text-muted-foreground'>
                  <Loader2 className='size-3.5 animate-spin' />
                  <span>Loading path...</span>
                </div>
              ) : (
                <span className='font-mono text-xs text-muted-foreground break-all'>
                  {storagePath || 'Direct stream storage'}
                </span>
              )}
              {mediaUrl && (
                <a href={mediaUrl} target='_blank' rel='noreferrer' download>
                  <Button
                    variant='outline'
                    size='sm'
                    className='gap-1 text-xs shrink-0'
                  >
                    <Download className='size-3.5' />
                    <span>Download</span>
                  </Button>
                </a>
              )}
            </div>
          </div>

          {/* System Error if Failed */}
          {currentJob.status === 'failed' && (
            <div className='space-y-1.5'>
              <span className='text-xs font-semibold text-rose-600 dark:text-rose-400 flex items-center gap-1.5'>
                <AlertCircle className='size-3.5' />
                <span>{t('rawError')}</span>
              </span>
              <div className='rounded-xl border border-rose-500/20 bg-rose-500/10 p-3 font-mono text-xs text-rose-600 dark:text-rose-400 select-text break-all'>
                {isLoadingDetail ? (
                  <div className='flex items-center gap-2'>
                    <Loader2 className='size-3.5 animate-spin' />
                    <span>Loading error details...</span>
                  </div>
                ) : (
                  errorMsg || 'No detailed error reported.'
                )}
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
              {resultJson && (
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
              {isLoadingDetail ? (
                <div className='flex h-full items-center justify-center gap-2 text-zinc-500'>
                  <Loader2 className='size-4 animate-spin' />
                  <span>Loading full payload...</span>
                </div>
              ) : resultJson ? (
                <pre className='whitespace-pre-wrap break-all leading-relaxed'>
                  {(() => {
                    try {
                      return JSON.stringify(JSON.parse(resultJson), null, 2);
                    } catch {
                      return resultJson;
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
