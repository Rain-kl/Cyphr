// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
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
  type JobDTO,
  type JobListDTO,
} from '@/lib/services/transcribe';
import {
  CheckCircle2,
  Clock,
  Eye,
  FileAudio,
  FileVideo,
  Loader2,
  RefreshCw,
  Search,
  Server,
  User,
  XCircle,
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

  const [selectedJob, setSelectedJob] = React.useState<JobDTO | null>(null);

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
        return <Badge variant='secondary'>{jobStatus}</Badge>;
    }
  };

  return (
    <div className='space-y-4'>
      {/* Filter Bar */}
      <div className='flex flex-wrap items-center gap-2.5'>
        <div className='relative w-48'>
          <Search className='absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground' />
          <Input
            type='search'
            value={keyword}
            onChange={(e) => {
              setKeyword(e.target.value);
              setPage(1);
            }}
            placeholder='Search filename...'
            className='h-8 pl-8 text-xs'
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
            className='h-8 w-32 text-xs'
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
          className='h-8 w-32 text-xs'
        />

        <Input
          type='number'
          value={nodeId}
          onChange={(e) => {
            setNodeId(e.target.value);
            setPage(1);
          }}
          placeholder={t('filterNode')}
          className='h-8 w-32 text-xs'
        />

        <Button
          variant='outline'
          size='sm'
          onClick={fetchJobs}
          disabled={isLoading}
          className='h-8 gap-1 px-2.5 text-xs'
        >
          <RefreshCw
            className={`size-3.5 ${isLoading ? 'animate-spin' : ''}`}
          />
          <span>Refresh</span>
        </Button>
      </div>

      {/* Panorama Jobs Table */}
      <div className='rounded-xl border bg-card shadow-sm overflow-hidden'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className='w-16'>{tTable('id')}</TableHead>
              <TableHead className='w-24'>{t('userCol')}</TableHead>
              <TableHead className='w-28'>{t('nodeCol')}</TableHead>
              <TableHead>{tTable('fileName')}</TableHead>
              <TableHead className='w-32'>{tTable('model')}</TableHead>
              <TableHead className='w-28'>{tTable('status')}</TableHead>
              <TableHead className='w-32'>{tTable('progress')}</TableHead>
              <TableHead className='w-24'>{tTable('duration')}</TableHead>
              <TableHead className='w-32'>{tTable('createdAt')}</TableHead>
              <TableHead className='w-20 text-right'>Action</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.items.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={10}
                  className='h-32 text-center text-muted-foreground'
                >
                  {isLoading ? tCommon('loading') : 'No platform jobs found.'}
                </TableCell>
              </TableRow>
            ) : (
              data.items.map((job) => (
                <TableRow
                  key={job.id}
                  onClick={() => setSelectedJob(job)}
                  className='cursor-pointer hover:bg-muted/50'
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
                      <span className='truncate font-medium text-xs'>
                        {job.original_file_name}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <code className='rounded bg-muted px-1.5 py-0.5 text-[11px] font-mono'>
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
                    <Button
                      variant='ghost'
                      size='icon'
                      className='size-7 text-muted-foreground hover:text-foreground'
                      onClick={(e) => {
                        e.stopPropagation();
                        setSelectedJob(job);
                      }}
                      aria-label='Inspect job'
                    >
                      <Eye className='size-3.5' />
                    </Button>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>

        {/* Pagination Footer */}
        {data.total > 0 && (
          <div className='flex items-center justify-between border-t px-4 py-3'>
            <p className='text-xs text-muted-foreground'>
              {page} / {totalPages} (Total: {data.total})
            </p>
            <div className='flex items-center gap-2'>
              <Button
                variant='outline'
                size='sm'
                disabled={page <= 1 || isLoading}
                onClick={() => setPage(page - 1)}
              >
                {tCommon('previousPage')}
              </Button>
              <Button
                variant='outline'
                size='sm'
                disabled={page >= totalPages || isLoading}
                onClick={() => setPage(page + 1)}
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
