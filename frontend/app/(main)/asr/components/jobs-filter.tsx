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
      <div className='flex flex-1 flex-wrap items-center gap-2'>
        <div className='relative w-full max-w-xs'>
          <Search className='absolute left-2.5 top-2.5 size-3 text-muted-foreground' />
          <Input
            type='search'
            value={keyword}
            onChange={(e) => onKeywordChange(e.target.value)}
            placeholder={t('searchPlaceholder')}
            className='h-8 pl-8 text-xs w-full shadow-none border-dashed bg-background'
          />
        </div>

        <Select value={status} onValueChange={onStatusChange}>
          <SelectTrigger
            className='h-8 w-32 border-dashed shadow-none text-xs bg-background'
            aria-label={t('allStatus')}
          >
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
          size='sm'
          onClick={onRefresh}
          disabled={isLoading}
          className='h-8 border-dashed text-xs shadow-none px-2.5 gap-1'
          aria-label={t('refresh')}
        >
          <RefreshCw className={`size-3 ${isLoading ? 'animate-spin' : ''}`} />
          <span>{t('refresh')}</span>
        </Button>
      </div>

      <Button
        size='sm'
        onClick={onOpenNewJob}
        className='h-8 text-xs shadow-none shrink-0 gap-1.5'
      >
        <Plus className='size-3.5' />
        <span>{tGeneral('newJob')}</span>
      </Button>
    </div>
  );
}
