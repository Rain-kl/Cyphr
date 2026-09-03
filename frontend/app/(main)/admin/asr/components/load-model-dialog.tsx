// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  AdminTranscribeService,
  type ModelDTO,
  type NodeDTO,
} from '@/lib/services/transcribe';
import { Cpu } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

interface LoadModelDialogProps {
  node: NodeDTO | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess: () => void;
}

export function LoadModelDialog({
  node,
  open,
  onOpenChange,
  onSuccess,
}: LoadModelDialogProps) {
  const t = useTranslations('adminAsr.models');

  const [models, setModels] = React.useState<ModelDTO[]>([]);
  const [selectedModel, setSelectedModel] = React.useState<string>('');
  const [isSubmitting, setIsSubmitting] = React.useState<boolean>(false);

  React.useEffect(() => {
    if (open) {
      AdminTranscribeService.listAllModels()
        .then((list) => {
          setModels(list);
          if (list.length > 0 && !selectedModel) {
            setSelectedModel(list[0].name);
          }
        })
        .catch(() => {});
    }
  }, [open, selectedModel]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!node || !selectedModel) return;

    try {
      setIsSubmitting(true);
      await AdminTranscribeService.loadModel(node.id, selectedModel);
      toast.success(t('dispatchSuccess'));
      onOpenChange(false);
      onSuccess();
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || 'Failed to dispatch model load');
    } finally {
      setIsSubmitting(false);
    }
  };

  if (!node) return null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2 text-lg font-semibold'>
            <Cpu className='size-5 text-primary' />
            {t('dispatchTitle')}
          </DialogTitle>
          <DialogDescription>
            Target Node:{' '}
            <span className='font-mono font-semibold text-foreground'>
              {node.name}
            </span>
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className='space-y-4 pt-2'>
          <div className='space-y-1.5'>
            <Label htmlFor='load-model-select' className='text-xs font-medium'>
              {t('selectModel')}
            </Label>
            <Select value={selectedModel} onValueChange={setSelectedModel}>
              <SelectTrigger
                id='load-model-select'
                aria-label={t('selectModel')}
              >
                <SelectValue placeholder='Select model to load' />
              </SelectTrigger>
              <SelectContent>
                {models.map((m) => (
                  <SelectItem key={m.id} value={m.name}>
                    {m.name} ({m.task_type})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <DialogFooter className='pt-2'>
            <Button
              type='button'
              variant='outline'
              onClick={() => onOpenChange(false)}
              disabled={isSubmitting}
            >
              Cancel
            </Button>
            <Button type='submit' disabled={isSubmitting || !selectedModel}>
              {isSubmitting ? 'Dispatching...' : t('dispatchBtn')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
