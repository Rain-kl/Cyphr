// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { Card, CardContent } from '@/components/ui/card';
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
      bg: 'bg-primary/10',
    },
    {
      title: t('running'),
      value: running,
      icon: Clock,
      color: 'text-amber-500',
      bg: 'bg-amber-500/10',
    },
    {
      title: t('completed'),
      value: completed,
      icon: CheckCircle2,
      color: 'text-emerald-500',
      bg: 'bg-emerald-500/10',
    },
    {
      title: t('failed'),
      value: failed,
      icon: XCircle,
      color: 'text-rose-500',
      bg: 'bg-rose-500/10',
    },
  ];

  return (
    <div className='grid grid-cols-2 gap-4 md:grid-cols-4'>
      {stats.map((item) => {
        const Icon = item.icon;
        return (
          <Card key={item.title} className='border shadow-sm'>
            <CardContent className='flex items-center justify-between p-4'>
              <div>
                <p className='text-xs font-medium text-muted-foreground'>
                  {item.title}
                </p>
                <div className='mt-1 text-2xl font-bold tracking-tight'>
                  {item.value}
                </div>
              </div>
              <div className={`rounded-xl p-2.5 ${item.bg}`}>
                <Icon className={`size-5 ${item.color}`} />
              </div>
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}
