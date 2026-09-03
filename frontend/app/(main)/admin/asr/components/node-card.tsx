// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
} from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import type { NodeDTO } from '@/lib/services/transcribe';
import { Cpu, HardDrive, Server, Trash2, X } from 'lucide-react';
import { useTranslations } from 'next-intl';

interface NodeCardProps {
  node: NodeDTO;
  onOpenDetails: (node: NodeDTO) => void;
  onOpenLoadModel: (node: NodeDTO) => void;
  onUnloadModel: (nodeId: number, modelName: string) => void;
  onDeleteNode: (nodeId: number) => void;
}

export function NodeCard({
  node,
  onOpenDetails,
  onOpenLoadModel,
  onUnloadModel,
  onDeleteNode,
}: NodeCardProps) {
  const t = useTranslations('adminAsr.nodes');
  const tCommon = useTranslations('common');

  const sys = node.system;

  const formatRAM = (used?: number, total?: number) => {
    if (!used || !total) return '-';
    return `${(used / 1024).toFixed(1)} / ${(total / 1024).toFixed(1)} GB`;
  };

  return (
    <Card className='flex flex-col justify-between border shadow-sm transition-all hover:shadow-md'>
      <CardHeader className='pb-3'>
        <div className='flex items-start justify-between gap-2'>
          <div className='flex items-center gap-2.5 overflow-hidden'>
            <div className='rounded-lg bg-primary/10 p-2 text-primary shrink-0'>
              <Server className='size-5' />
            </div>
            <div className='min-w-0'>
              <h3 className='truncate font-semibold text-base leading-tight'>
                {node.name}
              </h3>
              <p className='text-xs font-mono text-muted-foreground mt-0.5'>
                {node.last_ip || 'No IP'} • #{node.id}
              </p>
            </div>
          </div>

          {node.is_online ? (
            <Badge
              variant='outline'
              className='gap-1 border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 shrink-0'
            >
              <span className='size-1.5 rounded-full bg-emerald-500 animate-pulse' />
              <span>{t('statusOnline')}</span>
            </Badge>
          ) : (
            <Badge
              variant='outline'
              className='gap-1 text-muted-foreground shrink-0'
            >
              <span className='size-1.5 rounded-full bg-muted-foreground' />
              <span>{t('statusOffline')}</span>
            </Badge>
          )}
        </div>
      </CardHeader>

      <CardContent className='space-y-3.5 pb-4'>
        {/* Resource Gauges */}
        <div className='space-y-2.5 rounded-lg bg-muted/20 p-3 text-xs'>
          {/* CPU Bar */}
          <div className='space-y-1'>
            <div className='flex justify-between font-medium text-muted-foreground'>
              <span className='flex items-center gap-1'>
                <Cpu className='size-3 text-blue-500' />
                <span>{t('cpu')}</span>
              </span>
              <span className='font-mono text-foreground font-semibold'>
                {sys ? `${sys.cpu_percent.toFixed(0)}%` : '0%'}
              </span>
            </div>
            <Progress value={sys?.cpu_percent || 0} className='h-1.5' />
          </div>

          {/* RAM Bar */}
          <div className='space-y-1'>
            <div className='flex justify-between font-medium text-muted-foreground'>
              <span className='flex items-center gap-1'>
                <HardDrive className='size-3 text-indigo-500' />
                <span>{t('ram')}</span>
              </span>
              <span className='font-mono text-foreground font-semibold'>
                {sys?.ram_used_mb && sys?.ram_total_mb
                  ? formatRAM(sys.ram_used_mb, sys.ram_total_mb)
                  : 'N/A'}
              </span>
            </div>
            <Progress value={sys?.ram_percent || 0} className='h-1.5' />
          </div>
        </div>

        {/* Loaded Models Badges */}
        <div className='space-y-1.5'>
          <div className='flex items-center justify-between text-xs text-muted-foreground'>
            <span>{t('loadedModels')}</span>
            <span className='font-mono'>
              {node.running_jobs ? `${node.running_jobs} active jobs` : 'idle'}
            </span>
          </div>

          {node.loaded_models && node.loaded_models.length > 0 ? (
            <div className='flex flex-wrap gap-1'>
              {node.loaded_models.map((m) => (
                <Badge
                  key={m}
                  variant='secondary'
                  className='gap-1 pr-1 text-[11px] font-mono'
                >
                  <span>{m}</span>
                  {node.is_online && (
                    <button
                      type='button'
                      onClick={() => onUnloadModel(node.id, m)}
                      className='rounded-full p-0.5 hover:bg-muted-foreground/20 text-muted-foreground hover:text-foreground'
                      aria-label={`Unload ${m}`}
                    >
                      <X className='size-3' />
                    </button>
                  )}
                </Badge>
              ))}
            </div>
          ) : (
            <p className='text-xs text-muted-foreground italic'>
              {t('noModels')}
            </p>
          )}
        </div>
      </CardContent>

      <CardFooter className='border-t pt-3 flex items-center justify-between gap-2'>
        <div className='flex items-center gap-2'>
          <Button
            variant='outline'
            size='sm'
            disabled={!node.is_online}
            onClick={() => onOpenLoadModel(node)}
            className='h-8 text-xs'
          >
            {t('loadModelAction')}
          </Button>
          <Button
            variant='ghost'
            size='sm'
            onClick={() => onOpenDetails(node)}
            className='h-8 text-xs'
          >
            {t('viewDetails')}
          </Button>
        </div>

        {/* Delete Node Alert Dialog */}
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button
              variant='ghost'
              size='icon'
              className='size-8 text-muted-foreground hover:text-rose-600 hover:bg-rose-500/10'
              aria-label={t('deleteNode')}
            >
              <Trash2 className='size-4' />
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t('deleteConfirmTitle')}</AlertDialogTitle>
              <AlertDialogDescription>
                {t('deleteConfirmDesc')}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{tCommon('cancel')}</AlertDialogCancel>
              <AlertDialogAction
                onClick={() => onDeleteNode(node.id)}
                className='bg-rose-600 hover:bg-rose-700 text-white'
              >
                {tCommon('delete')}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </CardFooter>
    </Card>
  );
}
