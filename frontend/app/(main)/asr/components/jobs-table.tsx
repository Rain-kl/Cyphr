// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type { JobDTO } from '@/lib/services/transcribe';
import {
  ArrowRight,
  CheckCircle2,
  Clock,
  FileAudio,
  FileVideo,
  Loader2,
  XCircle,
} from 'lucide-react';
import { useTranslations } from 'next-intl';

interface JobsTableProps {
  jobs: JobDTO[];
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (newPage: number) => void;
  isLoading: boolean;
}

export function JobsTable({
  jobs,
  total,
  page,
  pageSize,
  onPageChange,
  isLoading,
}: JobsTableProps) {
  const router = useRouter();
  const t = useTranslations('asr.table');
  const tFilter = useTranslations('asr.filter');
  const tCommon = useTranslations('common');

  const totalPages = Math.ceil(total / pageSize) || 1;

  const formatDuration = (seconds?: number) => {
    if (!seconds || seconds <= 0) return '-';
    const m = Math.floor(seconds / 60);
    const s = Math.floor(seconds % 60);
    if (m === 0) return `${s}s`;
    return `${m}m ${s}s`;
  };

  const formatCreatedAt = (dateStr: string) => {
    try {
      const d = new Date(dateStr);
      return d.toLocaleString(undefined, {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return dateStr;
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'pending':
        return (
          <Badge
            variant='outline'
            className='gap-1 border-blue-500/30 bg-blue-500/10 text-blue-600 dark:text-blue-400'
          >
            <Clock className='size-3' />
            <span>{tFilter('statusPending')}</span>
          </Badge>
        );
      case 'running':
        return (
          <Badge
            variant='outline'
            className='gap-1 border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400'
          >
            <Loader2 className='size-3 animate-spin' />
            <span>{tFilter('statusRunning')}</span>
          </Badge>
        );
      case 'completed':
        return (
          <Badge
            variant='outline'
            className='gap-1 border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
          >
            <CheckCircle2 className='size-3' />
            <span>{tFilter('statusCompleted')}</span>
          </Badge>
        );
      case 'failed':
        return (
          <Badge
            variant='outline'
            className='gap-1 border-rose-500/30 bg-rose-500/10 text-rose-600 dark:text-rose-400'
          >
            <XCircle className='size-3' />
            <span>{tFilter('statusFailed')}</span>
          </Badge>
        );
      default:
        return <Badge variant='secondary'>{status}</Badge>;
    }
  };

  return (
    <div className='rounded-xl border bg-card shadow-sm'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='w-20'>{t('id')}</TableHead>
            <TableHead>{t('fileName')}</TableHead>
            <TableHead className='w-36'>{t('model')}</TableHead>
            <TableHead className='w-32'>{t('status')}</TableHead>
            <TableHead className='w-36'>{t('progress')}</TableHead>
            <TableHead className='w-28'>{t('duration')}</TableHead>
            <TableHead className='w-40'>{t('createdAt')}</TableHead>
            <TableHead className='w-24 text-right'>{t('actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {jobs.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={8}
                className='h-36 text-center text-muted-foreground'
              >
                {isLoading ? tCommon('loading') : t('empty')}
              </TableCell>
            </TableRow>
          ) : (
            jobs.map((job) => (
              <TableRow
                key={job.id}
                onClick={() => router.push(`/asr/jobs/${job.id}`)}
                className='cursor-pointer hover:bg-muted/50'
              >
                <TableCell className='font-mono text-xs font-semibold'>
                  #{job.id}
                </TableCell>
                <TableCell>
                  <div className='flex items-center gap-2 max-w-xs sm:max-w-md'>
                    <div className='rounded p-1 bg-muted text-muted-foreground shrink-0'>
                      {job.original_file_name.match(
                        /\.(mp4|mkv|mov|flv|webm)$/i,
                      ) ? (
                        <FileVideo className='size-4' />
                      ) : (
                        <FileAudio className='size-4' />
                      )}
                    </div>
                    <span className='truncate font-medium text-sm'>
                      {job.original_file_name}
                    </span>
                  </div>
                </TableCell>
                <TableCell>
                  <code className='rounded bg-muted px-1.5 py-0.5 text-xs'>
                    {job.model}
                  </code>
                </TableCell>
                <TableCell>{getStatusBadge(job.status)}</TableCell>
                <TableCell>
                  <div className='space-y-1'>
                    <Progress value={job.progress} className='h-1.5' />
                    <span className='text-[10px] text-muted-foreground font-mono'>
                      {job.progress}%
                    </span>
                  </div>
                </TableCell>
                <TableCell className='text-xs font-mono text-muted-foreground'>
                  {formatDuration(job.duration)}
                </TableCell>
                <TableCell className='text-xs text-muted-foreground whitespace-nowrap'>
                  {formatCreatedAt(job.created_at)}
                </TableCell>
                <TableCell className='text-right'>
                  <Link
                    href={`/asr/jobs/${job.id}`}
                    onClick={(e) => e.stopPropagation()}
                  >
                    <Button
                      variant='ghost'
                      size='sm'
                      className='h-8 gap-1 px-2'
                    >
                      <span className='text-xs'>{t('viewDetail')}</span>
                      <ArrowRight className='size-3.5' />
                    </Button>
                  </Link>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>

      {/* Pagination Footer */}
      {total > 0 && (
        <div className='flex items-center justify-between border-t px-4 py-3'>
          <p className='text-xs text-muted-foreground'>
            {page} / {totalPages} (Total: {total})
          </p>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              disabled={page <= 1 || isLoading}
              onClick={() => onPageChange(page - 1)}
            >
              {tCommon('previousPage')}
            </Button>
            <Button
              variant='outline'
              size='sm'
              disabled={page >= totalPages || isLoading}
              onClick={() => onPageChange(page + 1)}
            >
              {tCommon('nextPage')}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
