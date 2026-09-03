// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import Link from 'next/link';
import { Badge } from '@/components/ui/badge';
import type { JobDTO } from '@/lib/services/transcribe';
import {
  AlertCircle,
  ArrowLeft,
  CheckCircle2,
  Clock,
  Cpu,
  FileAudio,
  FileVideo,
  Globe,
  Loader2,
  XCircle,
} from 'lucide-react';
import { useTranslations } from 'next-intl';

import { ExportMenu } from './export-menu';

interface JobHeaderProps {
  job: JobDTO;
}

export function JobHeader({ job }: JobHeaderProps) {
  const t = useTranslations('asr.jobDetail');
  const tFilter = useTranslations('asr.filter');

  const formatDuration = (seconds?: number) => {
    if (!seconds || seconds <= 0) return '-';
    const m = Math.floor(seconds / 60);
    const s = Math.floor(seconds % 60);
    return `${m}m ${s}s`;
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'pending':
        return (
          <Badge
            variant='outline'
            className='gap-1 border-blue-500/30 bg-blue-500/10 text-blue-600 dark:text-blue-400'
          >
            <Clock className='size-3.5' />
            <span>{tFilter('statusPending')}</span>
          </Badge>
        );
      case 'running':
        return (
          <Badge
            variant='outline'
            className='gap-1 border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400'
          >
            <Loader2 className='size-3.5 animate-spin' />
            <span>{tFilter('statusRunning')}</span>
          </Badge>
        );
      case 'completed':
        return (
          <Badge
            variant='outline'
            className='gap-1 border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
          >
            <CheckCircle2 className='size-3.5' />
            <span>{tFilter('statusCompleted')}</span>
          </Badge>
        );
      case 'failed':
        return (
          <Badge
            variant='outline'
            className='gap-1 border-rose-500/30 bg-rose-500/10 text-rose-600 dark:text-rose-400'
          >
            <XCircle className='size-3.5' />
            <span>{tFilter('statusFailed')}</span>
          </Badge>
        );
      default:
        return <Badge variant='secondary'>{status}</Badge>;
    }
  };

  const isVideo = Boolean(
    job.original_file_name.match(/\.(mp4|mkv|mov|flv|webm)$/i),
  );

  return (
    <div className='space-y-4'>
      {/* Breadcrumb Navigation */}
      <div className='flex items-center gap-2 text-xs text-muted-foreground'>
        <Link
          href='/asr'
          className='flex items-center gap-1 hover:text-foreground transition-colors'
        >
          <ArrowLeft className='size-3.5' />
          <span>{t('backToList')}</span>
        </Link>
        <span>/</span>
        <span className='font-mono font-medium text-foreground'>#{job.id}</span>
      </div>

      {/* Main Header Container */}
      <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
        <div className='flex items-start gap-3'>
          <div className='rounded-xl bg-primary/10 p-2.5 text-primary shrink-0 mt-0.5'>
            {isVideo ? (
              <FileVideo className='size-6' />
            ) : (
              <FileAudio className='size-6' />
            )}
          </div>
          <div>
            <div className='flex flex-wrap items-center gap-2'>
              <h1 className='text-2xl font-semibold tracking-tight'>
                {job.original_file_name}
              </h1>
              {getStatusBadge(job.status)}
            </div>

            <div className='mt-1.5 flex flex-wrap items-center gap-3 text-xs text-muted-foreground'>
              <div className='flex items-center gap-1 font-mono'>
                <Cpu className='size-3.5' />
                <span>{job.model}</span>
              </div>
              <span>•</span>
              <div className='flex items-center gap-1'>
                <Globe className='size-3.5' />
                <span className='capitalize'>{job.language || 'Auto'}</span>
              </div>
              {job.duration ? (
                <>
                  <span>•</span>
                  <div className='flex items-center gap-1'>
                    <Clock className='size-3.5' />
                    <span>{formatDuration(job.duration)}</span>
                  </div>
                </>
              ) : null}
            </div>
          </div>
        </div>

        {/* Action Group */}
        <div className='flex items-center gap-2 self-start sm:self-center'>
          <ExportMenu job={job} />
        </div>
      </div>

      {/* Failure Alert Banner */}
      {job.status === 'failed' && (
        <div className='flex items-start gap-3 rounded-xl border border-rose-500/20 bg-rose-500/10 p-4 text-rose-600 dark:text-rose-400'>
          <AlertCircle className='size-5 shrink-0 mt-0.5' />
          <div className='space-y-1 text-sm'>
            <p className='font-semibold'>{t('errorTitle')}</p>
            <p className='font-mono text-xs opacity-90'>
              {job.error_msg || 'Unknown transcription execution error'}
            </p>
          </div>
        </div>
      )}
    </div>
  );
}
