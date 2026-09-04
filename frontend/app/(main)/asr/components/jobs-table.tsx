// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Progress } from '@/components/ui/progress';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { TranscribeService, type JobDTO } from '@/lib/services/transcribe';
import { ArrowRight, FileAudio, FileVideo, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

interface JobsTableProps {
  jobs: JobDTO[];
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (newPage: number) => void;
  isLoading: boolean;
  onRefresh?: () => void;
}

export function JobsTable({
  jobs,
  total,
  page,
  pageSize,
  onPageChange,
  isLoading,
  onRefresh,
}: JobsTableProps) {
  const router = useRouter();
  const t = useTranslations('asr.table');
  const tFilter = useTranslations('asr.filter');
  const tCommon = useTranslations('common');

  const [selectedIds, setSelectedIds] = React.useState<Set<string | number>>(
    new Set(),
  );
  const [deleteDialogOpen, setDeleteDialogOpen] = React.useState(false);
  const [jobToDelete, setJobToDelete] = React.useState<string | number | null>(
    null,
  );
  const [isDeleting, setIsDeleting] = React.useState(false);

  // Clear selections when page changes or jobs change
  React.useEffect(() => {
    setSelectedIds(new Set());
  }, [page, jobs]);

  const totalPages = Math.ceil(total / pageSize) || 1;

  const handleSelectAll = (checked: boolean) => {
    if (checked) {
      setSelectedIds(new Set(jobs.map((j) => j.id)));
    } else {
      setSelectedIds(new Set());
    }
  };

  const handleSelectOne = (id: string | number, checked: boolean) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (checked) {
        next.add(id);
      } else {
        next.delete(id);
      }
      return next;
    });
  };

  const isAllSelected =
    jobs.length > 0 && jobs.every((j) => selectedIds.has(j.id));
  const isPartiallySelected = selectedIds.size > 0 && !isAllSelected;

  const handleDeleteConfirm = async () => {
    const idsToDelete = jobToDelete ? [jobToDelete] : Array.from(selectedIds);
    if (idsToDelete.length === 0) return;

    try {
      setIsDeleting(true);
      await TranscribeService.deleteJobs(idsToDelete);
      toast.success(t('deleteSuccess'));
      setSelectedIds(new Set());
      setJobToDelete(null);
      setDeleteDialogOpen(false);
      if (onRefresh) onRefresh();
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || t('deleteFailed'));
    } finally {
      setIsDeleting(false);
    }
  };

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
            {status}
          </Badge>
        );
    }
  };

  return (
    <div className='space-y-3'>
      {/* Batch Action Bar */}
      {selectedIds.size > 0 && (
        <div className='flex items-center justify-between rounded-lg border border-dashed bg-muted/40 px-4 py-2 text-xs'>
          <div className='flex items-center gap-2'>
            <span className='font-medium text-foreground'>
              {t('selectedCount', { count: selectedIds.size })}
            </span>
          </div>
          <div className='flex items-center gap-2'>
            <Button
              variant='destructive'
              size='sm'
              onClick={() => {
                setJobToDelete(null);
                setDeleteDialogOpen(true);
              }}
              className='h-7 gap-1.5 px-2.5 text-xs shadow-none'
            >
              <Trash2 className='size-3.5' />
              <span>{t('deleteSelected', { count: selectedIds.size })}</span>
            </Button>
            <Button
              variant='ghost'
              size='sm'
              onClick={() => setSelectedIds(new Set())}
              className='h-7 px-2 text-xs'
            >
              取消选择
            </Button>
          </div>
        </div>
      )}

      <div className='border border-dashed shadow-none rounded-lg overflow-hidden bg-background'>
        <Table className='w-full caption-bottom text-sm min-w-full'>
          <TableHeader className='bg-muted/40'>
            <TableRow className='border-dashed hover:bg-transparent'>
              <TableHead className='w-10 px-3'>
                <Checkbox
                  checked={
                    isAllSelected ||
                    (isPartiallySelected ? 'indeterminate' : false)
                  }
                  onCheckedChange={handleSelectAll}
                  aria-label='全选当前页'
                />
              </TableHead>
              <TableHead className='w-20 text-xs font-semibold'>
                {t('id')}
              </TableHead>
              <TableHead className='text-xs font-semibold'>
                {t('fileName')}
              </TableHead>
              <TableHead className='w-36 text-xs font-semibold'>
                {t('model')}
              </TableHead>
              <TableHead className='w-32 text-xs font-semibold'>
                {t('status')}
              </TableHead>
              <TableHead className='w-36 text-xs font-semibold'>
                {t('progress')}
              </TableHead>
              <TableHead className='w-28 text-xs font-semibold'>
                {t('duration')}
              </TableHead>
              <TableHead className='w-40 text-xs font-semibold'>
                {t('createdAt')}
              </TableHead>
              <TableHead className='w-28 text-xs font-semibold text-right'>
                {t('actions')}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {jobs.length === 0 ? (
              <TableRow className='border-dashed hover:bg-transparent'>
                <TableCell
                  colSpan={9}
                  className='h-36 text-center text-xs text-muted-foreground'
                >
                  {isLoading ? tCommon('loading') : t('empty')}
                </TableCell>
              </TableRow>
            ) : (
              jobs.map((job) => {
                const isChecked = selectedIds.has(job.id);
                return (
                  <TableRow
                    key={job.id}
                    onClick={() => router.push(`/asr/jobs/${job.id}`)}
                    className={`border-dashed hover:bg-muted/10 transition-colors cursor-pointer ${
                      isChecked ? 'bg-muted/20' : ''
                    }`}
                  >
                    <TableCell
                      className='px-3'
                      onClick={(e) => e.stopPropagation()}
                    >
                      <Checkbox
                        checked={isChecked}
                        onCheckedChange={(c) => handleSelectOne(job.id, !!c)}
                        aria-label={`选择任务 #${job.id}`}
                      />
                    </TableCell>
                    <TableCell className='font-mono text-xs font-semibold'>
                      #{job.id}
                    </TableCell>
                    <TableCell>
                      <div className='flex items-center gap-2 max-w-xs sm:max-w-md'>
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
                      <code className='rounded bg-muted px-1.5 py-0.5 text-xs font-mono'>
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
                      <div className='flex items-center justify-end gap-1'>
                        <Link
                          href={`/asr/jobs/${job.id}`}
                          onClick={(e) => e.stopPropagation()}
                        >
                          <Button
                            variant='ghost'
                            size='sm'
                            className='h-7 gap-1 px-2 text-xs rounded hover:bg-muted text-muted-foreground'
                          >
                            <span>{t('viewDetail')}</span>
                            <ArrowRight className='size-3' />
                          </Button>
                        </Link>
                        <Button
                          variant='ghost'
                          size='icon'
                          onClick={(e) => {
                            e.stopPropagation();
                            setJobToDelete(job.id);
                            setDeleteDialogOpen(true);
                          }}
                          className='h-7 w-7 text-muted-foreground hover:text-destructive hover:bg-destructive/10'
                          aria-label={`删除任务 #${job.id}`}
                        >
                          <Trash2 className='size-3.5' />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>

        {/* Pagination Footer */}
        {total > 0 && (
          <div className='flex items-center justify-between border-t border-dashed px-4 py-3 bg-muted/5'>
            <p className='text-xs text-muted-foreground'>
              {page} / {totalPages} (Total: {total})
            </p>
            <div className='flex items-center gap-2'>
              <Button
                variant='outline'
                size='sm'
                disabled={page <= 1 || isLoading}
                onClick={() => onPageChange(page - 1)}
                className='h-8 border-dashed text-xs shadow-none'
              >
                {tCommon('previousPage')}
              </Button>
              <Button
                variant='outline'
                size='sm'
                disabled={page >= totalPages || isLoading}
                onClick={() => onPageChange(page + 1)}
                className='h-8 border-dashed text-xs shadow-none'
              >
                {tCommon('nextPage')}
              </Button>
            </div>
          </div>
        )}
      </div>

      {/* Delete Confirmation Modal */}
      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('deleteConfirmTitle')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('deleteConfirmDesc', {
                count: jobToDelete ? 1 : selectedIds.size,
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>
              {tCommon('cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDeleteConfirm}
              disabled={isDeleting}
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
            >
              {isDeleting ? '正在删除...' : t('delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
