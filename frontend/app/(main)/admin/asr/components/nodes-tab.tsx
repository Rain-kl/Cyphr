// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
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
import { NodeDetailDrawer } from './node-detail-drawer';

export function NodesTab() {
  const t = useTranslations('adminAsr.nodes');

  const [nodes, setNodes] = React.useState<NodeDTO[]>([]);
  const [keyword, setKeyword] = React.useState('');
  const [isLoading, setIsLoading] = React.useState(false);

  const [createDialogOpen, setCreateDialogOpen] = React.useState(false);
  const [selectedNodeForDetails, setSelectedNodeForDetails] =
    React.useState<NodeDTO | null>(null);
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

  const handleUnloadModel = async (nodeId: number, modelName: string) => {
    try {
      await AdminTranscribeService.unloadModel(nodeId, modelName);
      toast.success(`Unload command dispatched for ${modelName}`);
      fetchNodes();
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || 'Failed to unload model');
    }
  };

  const handleDeleteNode = async (nodeId: number) => {
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
        <Card className='border shadow-sm'>
          <CardContent className='flex items-center justify-between p-4'>
            <div>
              <p className='text-xs font-medium text-muted-foreground'>
                {t('totalNodes')}
              </p>
              <div className='mt-1 text-2xl font-bold tracking-tight'>
                {metrics.total}
              </div>
            </div>
            <div className='rounded-xl p-2.5 bg-primary/10 text-primary'>
              <Server className='size-5' />
            </div>
          </CardContent>
        </Card>

        <Card className='border shadow-sm'>
          <CardContent className='flex items-center justify-between p-4'>
            <div>
              <p className='text-xs font-medium text-muted-foreground'>
                {t('onlineNodes')}
              </p>
              <div className='mt-1 text-2xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400'>
                {metrics.online}
              </div>
            </div>
            <div className='rounded-xl p-2.5 bg-emerald-500/10 text-emerald-500'>
              <Wifi className='size-5' />
            </div>
          </CardContent>
        </Card>

        <Card className='border shadow-sm'>
          <CardContent className='flex items-center justify-between p-4'>
            <div>
              <p className='text-xs font-medium text-muted-foreground'>
                {t('totalRunning')}
              </p>
              <div className='mt-1 text-2xl font-bold tracking-tight text-amber-600 dark:text-amber-400'>
                {metrics.runningJobs}
              </div>
            </div>
            <div className='rounded-xl p-2.5 bg-amber-500/10 text-amber-500'>
              <Layers className='size-5' />
            </div>
          </CardContent>
        </Card>

        <Card className='border shadow-sm'>
          <CardContent className='flex items-center justify-between p-4'>
            <div>
              <p className='text-xs font-medium text-muted-foreground'>
                {t('avgCpu')}
              </p>
              <div className='mt-1 text-2xl font-bold tracking-tight font-mono'>
                {metrics.avgCpu}%
              </div>
            </div>
            <div className='rounded-xl p-2.5 bg-blue-500/10 text-blue-500'>
              <Activity className='size-5' />
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Toolbar */}
      <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <div className='flex flex-1 items-center gap-3'>
          <div className='relative w-full max-w-xs'>
            <Search className='absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground' />
            <Input
              type='search'
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
              placeholder='Search node name...'
              className='pl-8'
            />
          </div>

          <Button
            variant='outline'
            size='icon'
            onClick={fetchNodes}
            disabled={isLoading}
            aria-label='Refresh'
          >
            <RefreshCw
              className={`size-4 ${isLoading ? 'animate-spin' : ''}`}
            />
          </Button>
        </div>

        <Button
          onClick={() => setCreateDialogOpen(true)}
          className='gap-1.5 shadow-sm'
        >
          <Plus className='size-4' />
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
              onOpenDetails={setSelectedNodeForDetails}
              onOpenLoadModel={setSelectedNodeForLoadModel}
              onUnloadModel={handleUnloadModel}
              onDeleteNode={handleDeleteNode}
            />
          ))}
        </div>
      )}

      {/* Dialogs and Drawers */}
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

      <NodeDetailDrawer
        node={selectedNodeForDetails}
        open={Boolean(selectedNodeForDetails)}
        onOpenChange={(v) => !v && setSelectedNodeForDetails(null)}
        onUnloadModel={handleUnloadModel}
      />
    </div>
  );
}
