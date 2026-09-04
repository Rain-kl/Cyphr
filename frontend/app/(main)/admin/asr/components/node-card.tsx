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
  onUnloadModel: (nodeId: string | number, modelName: string) => void;
  onDeleteNode: (nodeId: string | number) => void;
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
    <Card className='flex flex-col justify-between border-dashed shadow-none transition-colors hover:bg-muted/5'>
      <CardHeader className='pb-3'>
        <div className='flex items-start justify-between gap-2'>
          <div className='flex items-center gap-2.5 overflow-hidden'>
            <div className='rounded-lg bg-primary/10 p-2 text-primary shrink-0'>
              <Server className='size-4' />
            </div>
            <div className='min-w-0'>
              <p className='truncate font-semibold text-sm leading-tight text-foreground'>
                {node.name}
              </p>
              <p className='text-xs font-mono text-muted-foreground mt-0.5'>
                {node.last_ip || 'No IP'} • #{node.id}
              </p>
            </div>
          </div>

          {node.is_online ? (
            <Badge
              variant='outline'
              className='text-[10px] bg-emerald-500/10 border-emerald-500/20 text-emerald-600 rounded-full py-0 px-2 font-medium shrink-0'
            >
              <span className='size-1 bg-emerald-500 rounded-full mr-1.5 shrink-0 animate-pulse' />
              <span>{t('statusOnline')}</span>
            </Badge>
          ) : (
            <Badge
              variant='outline'
              className='text-[10px] text-muted-foreground rounded-full py-0 px-2 font-medium shrink-0'
            >
              <span className='size-1 bg-muted-foreground rounded-full mr-1.5 shrink-0' />
              <span>{t('statusOffline')}</span>
            </Badge>
          )}
        </div>
      </CardHeader>

      <CardContent className='space-y-3.5 pb-4'>
        {/* Resource Gauges */}
        <div className='space-y-2.5 rounded-lg border border-dashed bg-muted/10 p-3 text-xs'>
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
            <span className='font-mono text-[11px]'>
              {node.running_jobs ? `${node.running_jobs} active jobs` : 'idle'}
            </span>
          </div>

          {node.loaded_models && node.loaded_models.length > 0 ? (
            <div className='flex flex-wrap gap-1'>
              {node.loaded_models.map((m) => (
                <Badge
                  key={m}
                  variant='secondary'
                  className='gap-1 pr-1 text-[10px] font-mono border border-dashed shadow-none rounded-full py-0 px-2'
                >
                  <span>{m}</span>
                  {node.is_online && (
                    <button
                      type='button'
                      onClick={() => onUnloadModel(node.id, m)}
                      className='rounded-full p-0.5 hover:bg-muted-foreground/20 text-muted-foreground hover:text-foreground'
                      aria-label={`Unload ${m}`}
                    >
                      <X className='size-2.5' />
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

      <CardFooter className='border-t border-dashed pt-3 flex items-center justify-between gap-2'>
        <div className='flex items-center gap-2'>
          <Button
            variant='outline'
            size='sm'
            disabled={!node.is_online}
            onClick={() => onOpenLoadModel(node)}
            className='h-7 text-xs border-dashed shadow-none'
          >
            {t('loadModelAction')}
          </Button>
          <Button
            variant='ghost'
            size='sm'
            onClick={() => onOpenDetails(node)}
            className='h-7 text-xs rounded hover:bg-muted text-muted-foreground'
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
              className='h-7 w-7 rounded hover:bg-destructive/10 text-muted-foreground hover:text-destructive'
              aria-label={t('deleteNode')}
            >
              <Trash2 className='size-3.5' />
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
