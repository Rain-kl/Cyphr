// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import Link from 'next/link';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Progress } from '@/components/ui/progress';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  AdminTranscribeService,
  type JobListDTO,
  type JobSummaryDTO,
} from '@/lib/services/transcribe';
import {
  ExternalLink,
  Eye,
  FileAudio,
  FileVideo,
  RefreshCw,
  RotateCcw,
  Search,
  Server,
  User,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

import { JobDeepInspector } from './job-deep-inspector';

export function AllJobsTab() {
  const t = useTranslations('adminAsr.allJobs');
  const tTable = useTranslations('asr.table');
  const tFilter = useTranslations('asr.filter');
  const tCommon = useTranslations('common');

  const [data, setData] = React.useState<JobListDTO>({
    items: [],
    total: 0,
    page: 1,
    page_size: 20,
  });
  const [keyword, setKeyword] = React.useState('');
  const [status, setStatus] = React.useState('all');
  const [userId, setUserId] = React.useState('');
  const [nodeId, setNodeId] = React.useState('');
  const [page, setPage] = React.useState(1);
  const [isLoading, setIsLoading] = React.useState(false);
  const [retryingId, setRetryingId] = React.useState<string | number | null>(
    null,
  );

  const [selectedJob, setSelectedJob] = React.useState<JobSummaryDTO | null>(
    null,
  );

  const handleRetry = async (jobId: string | number, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      setRetryingId(jobId);
      await AdminTranscribeService.retryJob(jobId);
      toast.success(t('retrySuccess'));
      fetchJobs();
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || t('retryFailed'));
    } finally {
      setRetryingId(null);
    }
  };

  const fetchJobs = React.useCallback(async () => {
    try {
      setIsLoading(true);
      const res = await AdminTranscribeService.listAllJobs({
        page,
        page_size: 20,
        status: status === 'all' ? undefined : status,
        keyword: keyword.trim() || undefined,
        user_id: userId ? Number(userId) : undefined,
        node_id: nodeId ? Number(nodeId) : undefined,
      });
      setData(res);
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || 'Failed to fetch platform jobs');
    } finally {
      setIsLoading(false);
    }
  }, [page, status, keyword, userId, nodeId]);

  React.useEffect(() => {
    fetchJobs();
  }, [fetchJobs]);

  const totalPages = Math.ceil(data.total / 20) || 1;

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
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return dateStr;
    }
  };

  const getStatusBadge = (jobStatus: string) => {
    switch (jobStatus) {
      case 'pending':
        return (
          <Badge
            variant='outline'
            className='text-[10px] bg-amber-500/10 border-amber-500/20 text-amber-600 rounded-full py-0 px-2 font-medium'
          >
            <span className='size-1 bg-amber-500 rounded-full mr-1.5 shrink-0' />
            <span>{tFilter('statusPending')}</span>
          </Badge>
        );
      case 'running':
        return (
          <Badge
            variant='outline'
            className='text-[10px] bg-blue-500/10 border-blue-500/20 text-blue-600 rounded-full py-0 px-2 font-medium'
          >
            <span className='size-1 bg-blue-500 rounded-full mr-1.5 shrink-0 animate-pulse' />
            <span>{tFilter('statusRunning')}</span>
          </Badge>
        );
      case 'completed':
        return (
          <Badge
            variant='outline'
            className='text-[10px] bg-emerald-500/10 border-emerald-500/20 text-emerald-600 rounded-full py-0 px-2 font-medium'
          >
            <span className='size-1 bg-emerald-500 rounded-full mr-1.5 shrink-0' />
            <span>{tFilter('statusCompleted')}</span>
          </Badge>
        );
      case 'failed':
        return (
          <Badge
            variant='outline'
            className='text-[10px] bg-destructive/10 border-destructive/20 text-destructive rounded-full py-0 px-2 font-medium'
          >
            <span className='size-1 bg-destructive rounded-full mr-1.5 shrink-0' />
            <span>{tFilter('statusFailed')}</span>
          </Badge>
        );
      default:
        return (
          <Badge
            variant='outline'
            className='text-[10px] rounded-full py-0 px-2 font-medium'
          >
            {jobStatus}
          </Badge>
        );
    }
  };

  return (
    <div className='space-y-4'>
      {/* Filter Bar */}
      <div className='flex flex-wrap items-center gap-2'>
        <div className='relative w-48'>
          <Search className='absolute left-2.5 top-2.5 size-3 text-muted-foreground' />
          <Input
            type='search'
            value={keyword}
            onChange={(e) => {
              setKeyword(e.target.value);
              setPage(1);
            }}
            placeholder='Search filename...'
            className='h-8 pl-8 text-xs w-full shadow-none border-dashed bg-background'
          />
        </div>

        <Select
          value={status}
          onValueChange={(val) => {
            setStatus(val);
            setPage(1);
          }}
        >
          <SelectTrigger
            className='h-8 w-32 border-dashed shadow-none text-xs bg-background'
            aria-label={tFilter('allStatus')}
          >
            <SelectValue placeholder={tFilter('allStatus')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>{tFilter('allStatus')}</SelectItem>
            <SelectItem value='pending'>{tFilter('statusPending')}</SelectItem>
            <SelectItem value='running'>{tFilter('statusRunning')}</SelectItem>
            <SelectItem value='completed'>
              {tFilter('statusCompleted')}
            </SelectItem>
            <SelectItem value='failed'>{tFilter('statusFailed')}</SelectItem>
            <SelectItem value='cancelled'>
              {tFilter('statusCancelled')}
            </SelectItem>
          </SelectContent>
        </Select>

        <Input
          type='number'
          value={userId}
          onChange={(e) => {
            setUserId(e.target.value);
            setPage(1);
          }}
          placeholder={t('filterUser')}
          className='h-8 w-32 border-dashed shadow-none text-xs bg-background'
        />

        <Input
          type='number'
          value={nodeId}
          onChange={(e) => {
            setNodeId(e.target.value);
            setPage(1);
          }}
          placeholder={t('filterNode')}
          className='h-8 w-32 border-dashed shadow-none text-xs bg-background'
        />

        <Button
          variant='outline'
          size='sm'
          onClick={fetchJobs}
          disabled={isLoading}
          className='h-8 border-dashed text-xs shadow-none px-2.5 gap-1'
        >
          <RefreshCw className={`size-3 ${isLoading ? 'animate-spin' : ''}`} />
          <span>Refresh</span>
        </Button>
      </div>

      {/* Panorama Jobs Table */}
      <div className='border border-dashed shadow-none rounded-lg overflow-hidden bg-background'>
        <Table className='w-full caption-bottom text-sm min-w-full'>
          <TableHeader className='bg-muted/40'>
            <TableRow className='border-dashed hover:bg-transparent'>
              <TableHead className='w-16 text-xs font-semibold'>
                {tTable('id')}
              </TableHead>
              <TableHead className='w-24 text-xs font-semibold'>
                {t('userCol')}
              </TableHead>
              <TableHead className='w-28 text-xs font-semibold'>
                {t('nodeCol')}
              </TableHead>
              <TableHead className='text-xs font-semibold'>
                {tTable('fileName')}
              </TableHead>
              <TableHead className='w-32 text-xs font-semibold'>
                {tTable('model')}
              </TableHead>
              <TableHead className='w-28 text-xs font-semibold'>
                {tTable('status')}
              </TableHead>
              <TableHead className='w-32 text-xs font-semibold'>
                {tTable('progress')}
              </TableHead>
              <TableHead className='w-24 text-xs font-semibold'>
                {tTable('duration')}
              </TableHead>
              <TableHead className='w-32 text-xs font-semibold'>
                {tTable('createdAt')}
              </TableHead>
              <TableHead className='w-28 text-xs font-semibold text-right'>
                Action
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.items.length === 0 ? (
              <TableRow className='border-dashed hover:bg-transparent'>
                <TableCell
                  colSpan={10}
                  className='h-32 text-center text-xs text-muted-foreground'
                >
                  {isLoading ? tCommon('loading') : 'No platform jobs found.'}
                </TableCell>
              </TableRow>
            ) : (
              data.items.map((job) => (
                <TableRow
                  key={job.id}
                  onClick={() => setSelectedJob(job)}
                  className='border-dashed hover:bg-muted/10 transition-colors cursor-pointer'
                >
                  <TableCell className='font-mono text-xs font-semibold'>
                    #{job.id}
                  </TableCell>
                  <TableCell className='font-mono text-xs text-muted-foreground'>
                    {job.user_id ? (
                      <span className='flex items-center gap-1'>
                        <User className='size-3 text-muted-foreground' />
                        <span>#{job.user_id}</span>
                      </span>
                    ) : (
                      'Anon'
                    )}
                  </TableCell>
                  <TableCell className='font-mono text-xs text-muted-foreground'>
                    {job.node_id ? (
                      <span className='flex items-center gap-1 text-primary'>
                        <Server className='size-3' />
                        <span>#{job.node_id}</span>
                      </span>
                    ) : (
                      '-'
                    )}
                  </TableCell>
                  <TableCell>
                    <div className='flex items-center gap-2 max-w-xs'>
                      <div className='rounded p-1 bg-muted text-muted-foreground shrink-0'>
                        {job.original_file_name.match(
                          /\.(mp4|mkv|mov|flv|webm)$/i,
                        ) ? (
                          <FileVideo className='size-3.5' />
                        ) : (
                          <FileAudio className='size-3.5' />
                        )}
                      </div>
                      <span className='truncate font-medium text-xs text-foreground'>
                        {job.original_file_name}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <code className='rounded bg-muted px-1.5 py-0.5 text-[11px] font-mono'>
                      {job.model}
                    </code>
                  </TableCell>
                  <TableCell>
                    <div className='flex items-center gap-1.5'>
                      {getStatusBadge(job.status)}
                      {typeof job.retry_count === 'number' &&
                        job.retry_count > 0 && (
                          <span
                            className='rounded bg-muted px-1 py-0.5 text-[9px] font-mono text-muted-foreground'
                            title={`Retried ${job.retry_count} time(s)`}
                          >
                            r:{job.retry_count}
                          </span>
                        )}
                    </div>
                  </TableCell>
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
                    <div className='flex items-center justify-end gap-1'>
                      {job.status === 'failed' && (
                        <Button
                          variant='ghost'
                          size='icon'
                          disabled={retryingId === job.id}
                          className='h-6 w-6 rounded hover:bg-rose-500/10 text-rose-600'
                          onClick={(e) => handleRetry(job.id, e)}
                          title={t('retryBtn')}
                          aria-label={t('retryBtn')}
                        >
                          <RotateCcw
                            className={`size-3 ${retryingId === job.id ? 'animate-spin' : ''}`}
                          />
                        </Button>
                      )}
                      <Link
                        href={`/asr/jobs/${job.id}`}
                        onClick={(e) => e.stopPropagation()}
                      >
                        <Button
                          variant='ghost'
                          size='icon'
                          className='h-6 w-6 rounded hover:bg-muted text-muted-foreground'
                          title={t('viewDetail')}
                          aria-label={t('viewDetail')}
                        >
                          <ExternalLink className='size-3' />
                        </Button>
                      </Link>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='h-6 w-6 rounded hover:bg-muted text-muted-foreground'
                        onClick={(e) => {
                          e.stopPropagation();
                          setSelectedJob(job);
                        }}
                        title={t('inspectorTitle')}
                        aria-label={t('inspectorTitle')}
                      >
                        <Eye className='size-3' />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>

        {/* Pagination Footer */}
        {data.total > 0 && (
          <div className='flex items-center justify-between border-t border-dashed px-4 py-3 bg-muted/5'>
            <p className='text-xs text-muted-foreground'>
              {page} / {totalPages} (Total: {data.total})
            </p>
            <div className='flex items-center gap-2'>
              <Button
                variant='outline'
                size='sm'
                disabled={page <= 1 || isLoading}
                onClick={() => setPage(page - 1)}
                className='h-8 border-dashed text-xs shadow-none'
              >
                {tCommon('previousPage')}
              </Button>
              <Button
                variant='outline'
                size='sm'
                disabled={page >= totalPages || isLoading}
                onClick={() => setPage(page + 1)}
                className='h-8 border-dashed text-xs shadow-none'
              >
                {tCommon('nextPage')}
              </Button>
            </div>
          </div>
        )}
      </div>

      {/* Deep Inspector Drawer */}
      <JobDeepInspector
        job={selectedJob}
        open={Boolean(selectedJob)}
        onOpenChange={(v) => !v && setSelectedJob(null)}
      />
    </div>
  );
}
