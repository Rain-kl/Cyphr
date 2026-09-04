// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { RequireAuth } from '@/components/auth/require-auth';
import { TranscribeService, type JobListDTO } from '@/lib/services/transcribe';
import { AudioWaveform } from 'lucide-react';
import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

import { JobsFilter } from './components/jobs-filter';
import { JobsTable } from './components/jobs-table';
import { NewJobDialog } from './components/new-job-dialog';
import { StatsCards } from './components/stats-cards';

export default function ASRDashboardPage() {
  const t = useTranslations('asr');

  const [jobsData, setJobsData] = React.useState<JobListDTO>({
    items: [],
    total: 0,
    page: 1,
    page_size: 20,
  });
  const [keyword, setKeyword] = React.useState<string>('');
  const [status, setStatus] = React.useState<string>('all');
  const [page, setPage] = React.useState<number>(1);
  const [isLoading, setIsLoading] = React.useState<boolean>(false);
  const [dialogOpen, setDialogOpen] = React.useState<boolean>(false);

  const fetchJobs = React.useCallback(async () => {
    try {
      setIsLoading(true);
      const res = await TranscribeService.listMyJobs({
        page,
        page_size: 20,
        status: status === 'all' ? undefined : status,
        keyword: keyword.trim() || undefined,
      });
      setJobsData(res);
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || 'Failed to load jobs');
    } finally {
      setIsLoading(false);
    }
  }, [page, status, keyword]);

  React.useEffect(() => {
    fetchJobs();
  }, [fetchJobs]);

  // Auto-refresh when any job is active (pending or running)
  React.useEffect(() => {
    const hasActiveJobs = jobsData.items.some(
      (j) => j.status === 'pending' || j.status === 'running',
    );
    if (!hasActiveJobs) return;

    const timer = setInterval(() => {
      TranscribeService.listMyJobs({
        page,
        page_size: 20,
        status: status === 'all' ? undefined : status,
        keyword: keyword.trim() || undefined,
      })
        .then(setJobsData)
        .catch(() => {});
    }, 4000);

    return () => clearInterval(timer);
  }, [jobsData.items, page, status, keyword]);

  // Compute stat metrics
  const stats = React.useMemo(() => {
    let running = 0;
    let completed = 0;
    let failed = 0;
    for (const item of jobsData.items) {
      if (item.status === 'running' || item.status === 'pending') running++;
      else if (item.status === 'completed') completed++;
      else if (item.status === 'failed') failed++;
    }
    return {
      total: jobsData.total,
      running,
      completed,
      failed,
    };
  }, [jobsData]);

  return (
    <RequireAuth>
      <motion.div
        initial={{ opacity: 0, y: 15 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.35, ease: 'easeOut' }}
        className='flex w-full flex-col gap-6 py-6 px-1'
      >
        {/* Top Header */}
        <div className='flex flex-col md:flex-row md:items-center justify-between gap-4'>
          <div className='flex items-center gap-2'>
            <AudioWaveform className='size-5 text-primary' />
            <div>
              <h1 className='text-2xl font-semibold tracking-tight'>
                {t('title')}
              </h1>
              <p className='text-xs text-muted-foreground mt-0.5'>
                {t('subtitle')}
              </p>
            </div>
          </div>
        </div>

        {/* Stats Cards */}
        <StatsCards
          total={stats.total}
          running={stats.running}
          completed={stats.completed}
          failed={stats.failed}
        />

        {/* Filters and Actions Toolbar */}
        <JobsFilter
          keyword={keyword}
          onKeywordChange={(val) => {
            setKeyword(val);
            setPage(1);
          }}
          status={status}
          onStatusChange={(val) => {
            setStatus(val);
            setPage(1);
          }}
          onRefresh={fetchJobs}
          isLoading={isLoading}
          onOpenNewJob={() => setDialogOpen(true)}
        />

        {/* Jobs Data Table */}
        <JobsTable
          jobs={jobsData.items}
          total={jobsData.total}
          page={page}
          pageSize={20}
          onPageChange={setPage}
          isLoading={isLoading}
          onRefresh={fetchJobs}
        />

        {/* New Job Modal */}
        <NewJobDialog
          open={dialogOpen}
          onOpenChange={setDialogOpen}
          onSuccess={fetchJobs}
        />
      </motion.div>
    </RequireAuth>
  );
}
