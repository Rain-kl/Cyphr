// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { useParams } from 'next/navigation';
import { RequireAuth } from '@/components/auth/require-auth';
import { Spinner } from '@/components/ui/spinner';
import {
  TranscribeService,
  type JobDTO,
  type LogMessage,
} from '@/lib/services/transcribe';
import { motion } from 'motion/react';
import { toast } from 'sonner';

import { AudioPlayer, type AudioPlayerRef } from './audio-player';
import { JobHeader } from './job-header';
import { LiveTerminal } from './live-terminal';
import { TranscriptViewer } from './transcript-viewer';

export function JobDetailClient() {
  const params = useParams();
  const rawId = params?.id ? String(params.id) : '';

  const [job, setJob] = React.useState<JobDTO | null>(null);
  const [logs, setLogs] = React.useState<LogMessage[]>([]);
  const [isLoading, setIsLoading] = React.useState(true);
  const [currentTime, setCurrentTime] = React.useState(0);

  const audioPlayerRef = React.useRef<AudioPlayerRef | null>(null);

  const fetchJob = React.useCallback(async () => {
    if (!rawId || rawId === '0') {
      setIsLoading(false);
      return;
    }
    try {
      const data = await TranscribeService.getJobDetail(rawId);
      setJob(data);
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || 'Failed to fetch job detail');
    } finally {
      setIsLoading(false);
    }
  }, [rawId]);

  React.useEffect(() => {
    fetchJob();
  }, [fetchJob]);

  const jobStatus = job?.status;

  // Connect to SSE log stream when job is running or pending
  React.useEffect(() => {
    if (!rawId || rawId === '0') return;
    if (jobStatus && jobStatus !== 'running' && jobStatus !== 'pending') {
      return;
    }

    const cleanup = TranscribeService.streamJobLogs(
      rawId,
      (log) => {
        setLogs((prev) => [...prev, log]);
        if (log.progress) {
          setJob((prev) => (prev ? { ...prev, progress: log.progress } : null));
        }
      },
      (finishedJob) => {
        if (finishedJob) {
          // SSE finish event is a partial FinishMessage (no file name etc.): merge it
          // into the existing job instead of replacing, otherwise fields like
          // original_file_name become undefined and crash the header.
          setJob((prev) => (prev ? { ...prev, ...finishedJob } : finishedJob));
        } else {
          fetchJob();
        }
      },
    );

    return () => {
      cleanup();
    };
  }, [rawId, jobStatus, fetchJob]);

  const handleSeek = (seconds: number) => {
    audioPlayerRef.current?.seekTo(seconds);
  };

  if (isLoading && !job) {
    return (
      <RequireAuth>
        <div className='flex h-96 w-full items-center justify-center'>
          <Spinner className='size-8 text-primary' />
        </div>
      </RequireAuth>
    );
  }

  if (!job) {
    return (
      <RequireAuth>
        <div className='flex h-96 w-full flex-col items-center justify-center gap-2 text-muted-foreground'>
          <p className='text-sm'>Job #{rawId} not found.</p>
        </div>
      </RequireAuth>
    );
  }

  const isRunning = job.status === 'running' || job.status === 'pending';
  const isCompleted = job.status === 'completed';

  return (
    <RequireAuth>
      <motion.div
        initial={{ opacity: 0, y: 15 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.35, ease: 'easeOut' }}
        className='flex w-full flex-col gap-6 py-6 px-1'
      >
        {/* Job Header Info & Actions */}
        <JobHeader job={job} />

        {/* Studio Content Grid */}
        <div className='flex flex-col gap-6'>
          {/* If completed: Synchronized Audio Player */}
          {isCompleted && (
            <AudioPlayer
              ref={audioPlayerRef}
              jobId={job.id}
              mediaUrl={job.media_url}
              audioStoragePath={job.audio_storage_path}
              onTimeUpdate={setCurrentTime}
            />
          )}

          {/* Transcript Viewer (Segments + Full Text) */}
          {isCompleted && (
            <TranscriptViewer
              job={job}
              currentTime={currentTime}
              onSeek={handleSeek}
            />
          )}

          {/* Real-time SSE Execution Terminal */}
          <LiveTerminal
            logs={logs}
            isRunning={isRunning}
            onClear={() => setLogs([])}
          />
        </div>
      </motion.div>
    </RequireAuth>
  );
}
