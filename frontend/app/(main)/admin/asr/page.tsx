// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { Button } from '@/components/ui/button';
import { AudioWaveform, Sliders } from 'lucide-react';
import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';

import { AllJobsTab } from './components/all-jobs-tab';
import { SchedulingSettingsSheet } from './components/scheduling-settings-sheet';
import { RequireAuth } from '@/components/auth/require-auth';

export default function AdminASRPage() {
  const t = useTranslations('adminAsr');
  const [settingsOpen, setSettingsOpen] = React.useState(false);

  return (
    <RequireAuth>
      <motion.div
        initial={{ opacity: 0, y: 15 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.35, ease: 'easeOut' }}
        className='flex w-full flex-col gap-6 py-6 px-1'
      >
        {/* Title Header */}
        <div className='flex flex-col md:flex-row md:items-center justify-between gap-4'>
          <div className='flex items-center gap-2'>
            <AudioWaveform className='size-5 text-primary' />
            <div>
              <h1 className='text-2xl font-semibold tracking-tight'>
                {t('tabJobs')}
              </h1>
              <p className='text-xs text-muted-foreground mt-0.5'>
                {t('subtitle')}
              </p>
            </div>
          </div>
          <Button
            variant='outline'
            size='sm'
            onClick={() => setSettingsOpen(true)}
            className='h-8 gap-1.5 px-3 text-xs shrink-0'
          >
            <Sliders className='size-3.5 text-primary' />
            <span>{t('schedulingSettingsBtn')}</span>
          </Button>
        </div>

        {/* All Jobs Management */}
        <div className='w-full'>
          <AllJobsTab />
        </div>

        {/* Scheduling & Retry Settings Sheet */}
        <SchedulingSettingsSheet
          open={settingsOpen}
          onOpenChange={setSettingsOpen}
        />
      </motion.div>
    </RequireAuth>
  );
}
