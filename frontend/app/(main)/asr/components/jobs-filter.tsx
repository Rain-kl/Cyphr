// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Plus, RefreshCw, Search } from 'lucide-react';
import { useTranslations } from 'next-intl';

interface JobsFilterProps {
  keyword: string;
  onKeywordChange: (value: string) => void;
  status: string;
  onStatusChange: (value: string) => void;
  onRefresh: () => void;
  isLoading: boolean;
  onOpenNewJob: () => void;
}

export function JobsFilter({
  keyword,
  onKeywordChange,
  status,
  onStatusChange,
  onRefresh,
  isLoading,
  onOpenNewJob,
}: JobsFilterProps) {
  const t = useTranslations('asr.filter');
  const tGeneral = useTranslations('asr');

  return (
    <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
      <div className='flex flex-1 flex-wrap items-center gap-3'>
        <div className='relative w-full max-w-xs'>
          <Search className='absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground' />
          <Input
            type='search'
            value={keyword}
            onChange={(e) => onKeywordChange(e.target.value)}
            placeholder={t('searchPlaceholder')}
            className='pl-8'
          />
        </div>

        <Select value={status} onValueChange={onStatusChange}>
          <SelectTrigger className='w-36' aria-label={t('allStatus')}>
            <SelectValue placeholder={t('allStatus')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>{t('allStatus')}</SelectItem>
            <SelectItem value='pending'>{t('statusPending')}</SelectItem>
            <SelectItem value='running'>{t('statusRunning')}</SelectItem>
            <SelectItem value='completed'>{t('statusCompleted')}</SelectItem>
            <SelectItem value='failed'>{t('statusFailed')}</SelectItem>
            <SelectItem value='cancelled'>{t('statusCancelled')}</SelectItem>
          </SelectContent>
        </Select>

        <Button
          variant='outline'
          size='icon'
          onClick={onRefresh}
          disabled={isLoading}
          aria-label={t('refresh')}
        >
          <RefreshCw className={`size-4 ${isLoading ? 'animate-spin' : ''}`} />
        </Button>
      </div>

      <Button onClick={onOpenNewJob} className='shrink-0 gap-1.5 shadow-sm'>
        <Plus className='size-4' />
        <span>{tGeneral('newJob')}</span>
      </Button>
    </div>
  );
}
