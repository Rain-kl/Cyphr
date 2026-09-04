// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { CheckCircle2, Clock, FileAudio, XCircle } from 'lucide-react';
import { useTranslations } from 'next-intl';

interface StatsCardsProps {
  total: number;
  running: number;
  completed: number;
  failed: number;
}

export function StatsCards({
  total,
  running,
  completed,
  failed,
}: StatsCardsProps) {
  const t = useTranslations('asr.stats');

  const stats = [
    {
      title: t('total'),
      value: total,
      icon: FileAudio,
      color: 'text-primary',
      desc: 'All recorded jobs',
    },
    {
      title: t('running'),
      value: running,
      icon: Clock,
      color: 'text-amber-500',
      desc: 'Processing or queued',
      dot: 'bg-amber-500',
    },
    {
      title: t('completed'),
      value: completed,
      icon: CheckCircle2,
      color: 'text-emerald-500',
      desc: 'Finished transcription',
      dot: 'bg-emerald-500',
    },
    {
      title: t('failed'),
      value: failed,
      icon: XCircle,
      color: 'text-rose-500',
      desc: 'Execution errors',
      dot: 'bg-rose-500',
    },
  ];

  return (
    <div className='grid grid-cols-2 gap-4 md:grid-cols-4'>
      {stats.map((item) => {
        const Icon = item.icon;
        return (
          <Card key={item.title} className='border-dashed shadow-none'>
            <CardHeader className='flex flex-row items-center justify-between pb-2'>
              <span className='text-xs font-medium text-muted-foreground'>
                {item.title}
              </span>
              <Icon className={`size-4 ${item.color}`} />
            </CardHeader>
            <CardContent className='space-y-1'>
              <div className='text-2xl font-semibold tracking-tight'>
                {item.value}
              </div>
              <p className='text-[10px] text-muted-foreground flex items-center gap-1.5'>
                {item.dot && (
                  <span
                    className={`size-1.5 rounded-full ${item.dot} ${
                      item.value > 0 && item.title === t('running')
                        ? 'animate-pulse'
                        : ''
                    }`}
                  />
                )}
                <span>{item.desc}</span>
              </p>
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}
