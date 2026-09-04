// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { Switch } from '@/components/ui/switch';
import services from '@/lib/services';
import { Loader2, RotateCcw, Sliders } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

interface SchedulingSettingsSheetProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const CONFIG_KEY_MAX_RETRIES = 'transcribe.job_max_retries';
const CONFIG_KEY_RETRY_INTERVAL = 'transcribe.job_retry_interval_seconds';

export function SchedulingSettingsSheet({
  open,
  onOpenChange,
}: SchedulingSettingsSheetProps) {
  const t = useTranslations('adminAsr.scheduling');
  const tCommon = useTranslations('common');

  const [isLoading, setIsLoading] = React.useState(false);
  const [isSaving, setIsSaving] = React.useState(false);

  const [enableRetry, setEnableRetry] = React.useState(false);
  const [maxRetries, setMaxRetries] = React.useState(2);
  const [retryInterval, setRetryInterval] = React.useState(60);

  // Load configs when opened
  React.useEffect(() => {
    if (!open) return;

    let isCancelled = false;
    setIsLoading(true);

    Promise.allSettled([
      services.adminSystemConfig.getSystemConfig(CONFIG_KEY_MAX_RETRIES),
      services.adminSystemConfig.getSystemConfig(CONFIG_KEY_RETRY_INTERVAL),
    ]).then(([retriesRes, intervalRes]) => {
      if (isCancelled) return;

      if (retriesRes.status === 'fulfilled' && retriesRes.value) {
        const val = parseInt(retriesRes.value.value, 10);
        if (!isNaN(val) && val > 0) {
          setEnableRetry(true);
          setMaxRetries(val);
        } else {
          setEnableRetry(false);
          setMaxRetries(2);
        }
      } else {
        setEnableRetry(false);
        setMaxRetries(2);
      }

      if (intervalRes.status === 'fulfilled' && intervalRes.value) {
        const val = parseInt(intervalRes.value.value, 10);
        if (!isNaN(val) && val > 0) {
          setRetryInterval(val);
        } else {
          setRetryInterval(60);
        }
      } else {
        setRetryInterval(60);
      }

      setIsLoading(false);
    });

    return () => {
      isCancelled = true;
    };
  }, [open]);

  const handleSave = async () => {
    try {
      setIsSaving(true);
      const effectiveMaxRetries = enableRetry ? Math.max(1, maxRetries) : 0;
      const effectiveInterval = Math.max(5, retryInterval);

      const saveItem = async (key: string, value: string, desc: string) => {
        try {
          await services.adminSystemConfig.updateSystemConfig(key, { value });
        } catch {
          await services.adminSystemConfig.createSystemConfig({
            key,
            value,
            type: 'business',
            description: desc,
          });
        }
      };

      await Promise.all([
        saveItem(
          CONFIG_KEY_MAX_RETRIES,
          String(effectiveMaxRetries),
          'Maximum number of automatic retries for failed transcription jobs (0 = disabled)',
        ),
        saveItem(
          CONFIG_KEY_RETRY_INTERVAL,
          String(effectiveInterval),
          'Seconds to wait before retrying a failed transcription job',
        ),
      ]);

      toast.success(t('saveSuccess'));
      onOpenChange(false);
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || t('saveFailed'));
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full sm:max-w-md p-6 space-y-6 flex flex-col justify-between overflow-y-auto'>
        <div className='space-y-6'>
          <SheetHeader>
            <div className='flex items-center gap-2'>
              <Sliders className='size-5 text-primary' />
              <SheetTitle className='text-lg font-semibold'>
                {t('title')}
              </SheetTitle>
            </div>
            <SheetDescription>{t('description')}</SheetDescription>
          </SheetHeader>

          {isLoading ? (
            <div className='flex items-center justify-center py-12 text-muted-foreground'>
              <Loader2 className='size-6 animate-spin' />
            </div>
          ) : (
            <div className='space-y-6 text-sm'>
              {/* Enable Retry Switch */}
              <div className='flex items-center justify-between rounded-xl border p-4'>
                <div className='space-y-0.5'>
                  <Label
                    htmlFor='retry-switch'
                    className='font-medium cursor-pointer'
                  >
                    {t('enableRetry')}
                  </Label>
                  <p className='text-xs text-muted-foreground'>
                    {enableRetry
                      ? t('retryEnabledDesc')
                      : t('retryDisabledDesc')}
                  </p>
                </div>
                <Switch
                  id='retry-switch'
                  checked={enableRetry}
                  onCheckedChange={setEnableRetry}
                />
              </div>

              {/* Retry Parameters (only when enabled) */}
              {enableRetry && (
                <div className='rounded-xl border bg-muted/20 p-4 space-y-4'>
                  <div className='space-y-1.5'>
                    <Label
                      htmlFor='max-retries'
                      className='text-xs font-medium'
                    >
                      {t('maxRetries')}
                    </Label>
                    <Input
                      id='max-retries'
                      type='number'
                      min={1}
                      max={10}
                      value={maxRetries}
                      onChange={(e) =>
                        setMaxRetries(parseInt(e.target.value, 10) || 1)
                      }
                      className='h-9 text-xs'
                    />
                    <p className='text-[11px] text-muted-foreground'>
                      {t('maxRetriesHint')}
                    </p>
                  </div>

                  <div className='space-y-1.5'>
                    <Label
                      htmlFor='retry-interval'
                      className='text-xs font-medium'
                    >
                      {t('retryInterval')}
                    </Label>
                    <Input
                      id='retry-interval'
                      type='number'
                      min={5}
                      max={3600}
                      value={retryInterval}
                      onChange={(e) =>
                        setRetryInterval(parseInt(e.target.value, 10) || 5)
                      }
                      className='h-9 text-xs'
                    />
                    <p className='text-[11px] text-muted-foreground'>
                      {t('retryIntervalHint')}
                    </p>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        <SheetFooter className='pt-4 border-t flex flex-row items-center justify-end gap-2'>
          <Button
            variant='outline'
            size='sm'
            disabled={isSaving}
            onClick={() => onOpenChange(false)}
          >
            {tCommon('cancel')}
          </Button>
          <Button
            size='sm'
            disabled={isLoading || isSaving}
            onClick={handleSave}
            className='gap-1.5'
          >
            {isSaving ? (
              <Loader2 className='size-3.5 animate-spin' />
            ) : (
              <RotateCcw className='size-3.5' />
            )}
            <span>{tCommon('save')}</span>
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
