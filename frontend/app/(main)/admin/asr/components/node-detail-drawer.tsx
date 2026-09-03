// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import {
  AdminTranscribeService,
  type NodeDTO,
} from '@/lib/services/transcribe';
import {
  Activity,
  CheckCircle2,
  Cpu,
  HardDrive,
  Layers,
  Server,
  X,
  XCircle,
} from 'lucide-react';
import { useTranslations } from 'next-intl';

interface NodeDetailDrawerProps {
  node: NodeDTO | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onUnloadModel?: (nodeId: number, modelName: string) => void;
}

export function NodeDetailDrawer({
  node,
  open,
  onOpenChange,
  onUnloadModel,
}: NodeDetailDrawerProps) {
  const t = useTranslations('adminAsr');
  const tDrawer = useTranslations('adminAsr.nodeDrawer');

  const [detailedNode, setDetailedNode] = React.useState<NodeDTO | null>(null);

  React.useEffect(() => {
    if (open && node) {
      AdminTranscribeService.getNode(node.id)
        .then(setDetailedNode)
        .catch(() => setDetailedNode(node));
    }
  }, [open, node]);

  const activeNode = detailedNode || node;
  if (!activeNode) return null;

  const sys = activeNode.system;

  const formatMB = (mb?: number) => {
    if (!mb) return '0 MB';
    if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
    return `${mb} MB`;
  };

  const formatTimestamp = (ts?: string) => {
    if (!ts) return '-';
    try {
      return new Date(ts).toLocaleString();
    } catch {
      return ts;
    }
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full sm:max-w-md overflow-y-auto'>
        <SheetHeader>
          <div className='flex items-center gap-2'>
            <Server className='size-5 text-primary' />
            <SheetTitle className='text-lg font-semibold'>
              {activeNode.name}
            </SheetTitle>
          </div>
          <SheetDescription>{tDrawer('title')}</SheetDescription>
        </SheetHeader>

        <div className='mt-6 space-y-6 text-sm'>
          {/* Status and Identity */}
          <div className='rounded-xl border bg-muted/20 p-3.5 space-y-2.5'>
            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground'>Node ID</span>
              <span className='font-mono font-semibold'>#{activeNode.id}</span>
            </div>
            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground'>Status</span>
              {activeNode.is_online ? (
                <Badge
                  variant='outline'
                  className='gap-1 border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                >
                  <CheckCircle2 className='size-3' />
                  <span>{t('nodes.statusOnline')}</span>
                </Badge>
              ) : (
                <Badge
                  variant='outline'
                  className='gap-1 text-muted-foreground'
                >
                  <XCircle className='size-3' />
                  <span>{t('nodes.statusOffline')}</span>
                </Badge>
              )}
            </div>
            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground'>
                {tDrawer('tokenPrefix')}
              </span>
              <span className='font-mono text-xs'>
                {activeNode.token_prefix}...
              </span>
            </div>
            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground'>
                {tDrawer('lastIp')}
              </span>
              <span className='font-mono text-xs'>
                {activeNode.last_ip || '-'}
              </span>
            </div>
            <div className='flex items-center justify-between'>
              <span className='text-xs text-muted-foreground'>
                {tDrawer('lastSeen')}
              </span>
              <span className='text-xs text-muted-foreground'>
                {formatTimestamp(activeNode.last_seen_at)}
              </span>
            </div>
          </div>

          {/* Real-time Hardware Telemetry */}
          <div className='space-y-3.5'>
            <h4 className='text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5'>
              <Activity className='size-3.5 text-primary' />
              Hardware Metrics
            </h4>

            {/* CPU */}
            <div className='rounded-xl border p-3 space-y-2'>
              <div className='flex justify-between text-xs font-medium'>
                <span className='flex items-center gap-1.5'>
                  <Cpu className='size-3.5 text-blue-500' />
                  <span>CPU Load</span>
                </span>
                <span className='font-mono font-semibold'>
                  {sys ? `${sys.cpu_percent.toFixed(1)}%` : '0%'}
                </span>
              </div>
              <Progress value={sys ? sys.cpu_percent : 0} className='h-2' />
            </div>

            {/* RAM */}
            <div className='rounded-xl border p-3 space-y-2'>
              <div className='flex justify-between text-xs font-medium'>
                <span className='flex items-center gap-1.5'>
                  <HardDrive className='size-3.5 text-indigo-500' />
                  <span>RAM Memory</span>
                </span>
                <span className='font-mono font-semibold'>
                  {sys && sys.ram_used_mb && sys.ram_total_mb
                    ? `${formatMB(sys.ram_used_mb)} / ${formatMB(sys.ram_total_mb)} (${sys.ram_percent?.toFixed(0)}%)`
                    : 'N/A'}
                </span>
              </div>
              <Progress value={sys?.ram_percent || 0} className='h-2' />
            </div>

            {/* GPU (if available) */}
            {sys &&
              ((sys.gpu_percent !== undefined && sys.gpu_percent > 0) ||
                (sys.gpu_memory_total_mb && sys.gpu_memory_total_mb > 0)) && (
                <div className='rounded-xl border p-3 space-y-2'>
                  <div className='flex justify-between text-xs font-medium'>
                    <span className='flex items-center gap-1.5'>
                      <Layers className='size-3.5 text-amber-500' />
                      <span>GPU Utilization</span>
                    </span>
                    <span className='font-mono font-semibold'>
                      {sys.gpu_percent?.toFixed(1)}%
                    </span>
                  </div>
                  <Progress value={sys.gpu_percent || 0} className='h-2' />
                  {sys.gpu_memory_total_mb && sys.gpu_memory_total_mb > 0 && (
                    <p className='text-[11px] text-muted-foreground font-mono'>
                      VRAM: {formatMB(sys.gpu_memory_used_mb)} /{' '}
                      {formatMB(sys.gpu_memory_total_mb)}
                    </p>
                  )}
                </div>
              )}
          </div>

          {/* Running Jobs and Models */}
          <div className='space-y-3'>
            <h4 className='text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5'>
              <Layers className='size-3.5 text-primary' />
              Loaded Models & Tasks
            </h4>

            <div className='flex items-center justify-between rounded-xl border p-3'>
              <span className='text-xs text-muted-foreground'>
                {tDrawer('activeJobs')}
              </span>
              <Badge variant='secondary' className='font-mono'>
                {activeNode.running_jobs || 0}
              </Badge>
            </div>

            <div className='rounded-xl border p-3 space-y-2'>
              <span className='text-xs text-muted-foreground'>
                {t('nodes.loadedModels')}
              </span>
              {activeNode.loaded_models &&
              activeNode.loaded_models.length > 0 ? (
                <div className='flex flex-wrap gap-1.5 pt-1'>
                  {activeNode.loaded_models.map((m) => (
                    <Badge
                      key={m}
                      variant='outline'
                      className='gap-1.5 pr-1 font-mono text-xs'
                    >
                      <span>{m}</span>
                      {onUnloadModel && (
                        <button
                          type='button'
                          onClick={() => onUnloadModel(activeNode.id, m)}
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
                <p className='text-xs text-muted-foreground italic pt-1'>
                  {t('nodes.noModels')}
                </p>
              )}
            </div>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}
