// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { RequireAuth } from '@/components/auth/require-auth';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Cpu, Layers, Server } from 'lucide-react';
import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';

import { AllJobsTab } from './components/all-jobs-tab';
import { ModelsTab } from './components/models-tab';
import { NodesTab } from './components/nodes-tab';

export default function AdminASRPage() {
  const t = useTranslations('adminAsr');

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
            <Cpu className='size-5 text-primary' />
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

        {/* Admin Center Tabs */}
        <Tabs defaultValue='nodes' className='space-y-6'>
          <TabsList className='h-9 w-full sm:w-auto grid grid-cols-3 sm:inline-flex'>
            <TabsTrigger value='nodes' className='gap-2 text-xs'>
              <Server className='size-3.5' />
              <span>{t('tabNodes')}</span>
            </TabsTrigger>
            <TabsTrigger value='models' className='gap-2 text-xs'>
              <Cpu className='size-3.5' />
              <span>{t('tabModels')}</span>
            </TabsTrigger>
            <TabsTrigger value='jobs' className='gap-2 text-xs'>
              <Layers className='size-3.5' />
              <span>{t('tabJobs')}</span>
            </TabsTrigger>
          </TabsList>

          <TabsContent value='nodes' className='m-0 focus-visible:outline-none'>
            <NodesTab />
          </TabsContent>

          <TabsContent
            value='models'
            className='m-0 focus-visible:outline-none'
          >
            <ModelsTab />
          </TabsContent>

          <TabsContent value='jobs' className='m-0 focus-visible:outline-none'>
            <AllJobsTab />
          </TabsContent>
        </Tabs>
      </motion.div>
    </RequireAuth>
  );
}
