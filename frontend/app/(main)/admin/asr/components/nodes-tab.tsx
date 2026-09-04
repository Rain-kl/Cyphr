// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { useRouter } from 'next/navigation';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import {
  AdminTranscribeService,
  type NodeDTO,
} from '@/lib/services/transcribe';
import {
  Activity,
  Layers,
  Plus,
  RefreshCw,
  Search,
  Server,
  Wifi,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

import { CreateNodeDialog } from './create-node-dialog';
import { LoadModelDialog } from './load-model-dialog';
import { NodeCard } from './node-card';

export function NodesTab() {
  const router = useRouter();
  const t = useTranslations('adminAsr.nodes');

  const [nodes, setNodes] = React.useState<NodeDTO[]>([]);
  const [keyword, setKeyword] = React.useState('');
  const [isLoading, setIsLoading] = React.useState(false);

  const [createDialogOpen, setCreateDialogOpen] = React.useState(false);
  const [selectedNodeForLoadModel, setSelectedNodeForLoadModel] =
    React.useState<NodeDTO | null>(null);

  const fetchNodes = React.useCallback(async () => {
    try {
      setIsLoading(true);
      const res = await AdminTranscribeService.listNodes(
        keyword.trim() || undefined,
      );
      setNodes(res);
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || 'Failed to fetch worker nodes');
    } finally {
      setIsLoading(false);
    }
  }, [keyword]);

  React.useEffect(() => {
    fetchNodes();
  }, [fetchNodes]);

  // Periodic polling for node metrics (every 5 seconds)
  React.useEffect(() => {
    const timer = setInterval(() => {
      AdminTranscribeService.listNodes(keyword.trim() || undefined)
        .then(setNodes)
        .catch(() => {});
    }, 5000);
    return () => clearInterval(timer);
  }, [keyword]);

  const handleUnloadModel = async (
    nodeId: string | number,
    modelName: string,
  ) => {
    try {
      await AdminTranscribeService.unloadModel(nodeId, modelName);
      toast.success(`Unload command dispatched for ${modelName}`);
      fetchNodes();
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || 'Failed to unload model');
    }
  };

  const handleDeleteNode = async (nodeId: string | number) => {
    try {
      await AdminTranscribeService.deleteNode(nodeId);
      toast.success(t('deleteSuccess'));
      fetchNodes();
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || 'Failed to delete node');
    }
  };

  // Metrics computation
  const metrics = React.useMemo(() => {
    const total = nodes.length;
    let online = 0;
    let runningJobs = 0;
    let cpuSum = 0;
    for (const n of nodes) {
      if (n.is_online) {
        online++;
        if (n.system) {
          cpuSum += n.system.cpu_percent;
        }
      }
      runningJobs += n.running_jobs || 0;
    }
    const avgCpu = online > 0 ? (cpuSum / online).toFixed(1) : '0';
    return {
      total,
      online,
      runningJobs,
      avgCpu,
    };
  }, [nodes]);

  return (
    <div className='space-y-6'>
      {/* Top Metrics Row */}
      <div className='grid grid-cols-2 gap-4 md:grid-cols-4'>
        <Card className='border-dashed shadow-none'>
          <CardHeader className='flex flex-row items-center justify-between pb-2'>
            <span className='text-xs font-medium text-muted-foreground'>
              {t('totalNodes')}
            </span>
            <Server className='size-4 text-primary' />
          </CardHeader>
          <CardContent className='space-y-1'>
            <div className='text-2xl font-semibold tracking-tight'>
              {metrics.total}
            </div>
            <p className='text-[10px] text-muted-foreground'>
              Cluster workers registered
            </p>
          </CardContent>
        </Card>

        <Card className='border-dashed shadow-none'>
          <CardHeader className='flex flex-row items-center justify-between pb-2'>
            <span className='text-xs font-medium text-muted-foreground'>
              {t('onlineNodes')}
            </span>
            <Wifi className='size-4 text-emerald-500' />
          </CardHeader>
          <CardContent className='space-y-1'>
            <div className='text-2xl font-semibold tracking-tight text-emerald-600 dark:text-emerald-400'>
              {metrics.online}
            </div>
            <p className='text-[10px] text-muted-foreground flex items-center gap-1.5'>
              <span className='size-1.5 rounded-full bg-emerald-500 animate-pulse' />
              <span>Available for jobs</span>
            </p>
          </CardContent>
        </Card>

        <Card className='border-dashed shadow-none'>
          <CardHeader className='flex flex-row items-center justify-between pb-2'>
            <span className='text-xs font-medium text-muted-foreground'>
              {t('totalRunning')}
            </span>
            <Layers className='size-4 text-amber-500' />
          </CardHeader>
          <CardContent className='space-y-1'>
            <div className='text-2xl font-semibold tracking-tight text-amber-600 dark:text-amber-400'>
              {metrics.runningJobs}
            </div>
            <p className='text-[10px] text-muted-foreground flex items-center gap-1.5'>
              {metrics.runningJobs > 0 && (
                <span className='size-1.5 rounded-full bg-amber-500 animate-pulse' />
              )}
              <span>Active inference tasks</span>
            </p>
          </CardContent>
        </Card>

        <Card className='border-dashed shadow-none'>
          <CardHeader className='flex flex-row items-center justify-between pb-2'>
            <span className='text-xs font-medium text-muted-foreground'>
              {t('avgCpu')}
            </span>
            <Activity className='size-4 text-blue-500' />
          </CardHeader>
          <CardContent className='space-y-1'>
            <div className='text-2xl font-semibold tracking-tight font-mono'>
              {metrics.avgCpu}%
            </div>
            <p className='text-[10px] text-muted-foreground'>
              Across active nodes
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Toolbar */}
      <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <div className='flex flex-1 items-center gap-2'>
          <div className='relative w-full max-w-xs'>
            <Search className='absolute left-2.5 top-2.5 size-3 text-muted-foreground' />
            <Input
              type='search'
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
              placeholder='Search node name...'
              className='h-8 pl-8 text-xs w-full shadow-none border-dashed bg-background'
            />
          </div>

          <Button
            variant='outline'
            size='sm'
            onClick={fetchNodes}
            disabled={isLoading}
            className='h-8 border-dashed text-xs shadow-none px-2.5 gap-1'
            aria-label='Refresh'
          >
            <RefreshCw
              className={`size-3 ${isLoading ? 'animate-spin' : ''}`}
            />
            <span>Refresh</span>
          </Button>
        </div>

        <Button
          size='sm'
          onClick={() => setCreateDialogOpen(true)}
          className='h-8 text-xs shadow-none shrink-0 gap-1.5'
        >
          <Plus className='size-3.5' />
          <span>{t('addNode')}</span>
        </Button>
      </div>

      {/* Nodes Grid */}
      {nodes.length === 0 ? (
        <div className='flex h-48 w-full flex-col items-center justify-center rounded-xl border border-dashed text-center text-muted-foreground'>
          <Server className='size-8 opacity-40 mb-2' />
          <p className='text-sm font-medium'>No worker nodes registered yet.</p>
          <p className='text-xs mt-0.5'>
            Click &quot;Add Node&quot; to provision an agent worker.
          </p>
        </div>
      ) : (
        <div className='grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3'>
          {nodes.map((node) => (
            <NodeCard
              key={node.id}
              node={node}
              onOpenDetails={(n) => router.push(`/admin/nodes/${n.id}`)}
              onOpenLoadModel={setSelectedNodeForLoadModel}
              onUnloadModel={handleUnloadModel}
              onDeleteNode={handleDeleteNode}
            />
          ))}
        </div>
      )}

      {/* Dialogs */}
      <CreateNodeDialog
        open={createDialogOpen}
        onOpenChange={setCreateDialogOpen}
        onSuccess={fetchNodes}
      />

      <LoadModelDialog
        node={selectedNodeForLoadModel}
        open={Boolean(selectedNodeForLoadModel)}
        onOpenChange={(v) => !v && setSelectedNodeForLoadModel(null)}
        onSuccess={fetchNodes}
      />
    </div>
  );
}
