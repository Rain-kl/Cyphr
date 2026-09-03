// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { RequireAuth } from '@/components/auth/require-auth';
import { Server } from 'lucide-react';
import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';

import { NodesTab } from '../asr/components/nodes-tab';

export default function AdminNodesPage() {
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
            <Server className='size-5 text-primary' />
            <div>
              <h1 className='text-2xl font-semibold tracking-tight'>
                {t('tabNodes')}
              </h1>
              <p className='text-xs text-muted-foreground mt-0.5'>
                {t('nodesSubtitle') ||
                  '分布式推理 Agent 节点监控、实时硬件遥测与调度管理'}
              </p>
            </div>
          </div>
        </div>

        {/* Nodes Content */}
        <div className='w-full'>
          <NodesTab />
        </div>
      </motion.div>
    </RequireAuth>
  );
}
