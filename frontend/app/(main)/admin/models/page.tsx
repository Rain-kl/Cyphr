// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { RequireAuth } from '@/components/auth/require-auth';
import { Cpu } from 'lucide-react';
import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';

import { ModelsTab } from '../asr/components/models-tab';

export default function AdminModelsPage() {
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
                {t('tabModels')}
              </h1>
              <p className='text-xs text-muted-foreground mt-0.5'>
                {t('modelsSubtitle') ||
                  '平台语音识别模型库、全局可用性开关与节点热调度分发'}
              </p>
            </div>
          </div>
        </div>

        {/* Models Content */}
        <div className='w-full'>
          <ModelsTab />
        </div>
      </motion.div>
    </RequireAuth>
  );
}
