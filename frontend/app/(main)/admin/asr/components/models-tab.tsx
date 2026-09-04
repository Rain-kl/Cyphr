// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  AdminTranscribeService,
  type ModelDTO,
  type NodeDTO,
} from '@/lib/services/transcribe';
import { Cpu, RefreshCw, Search, Send, Sparkles } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

export function ModelsTab() {
  const t = useTranslations('adminAsr.models');
  const tCommon = useTranslations('common');

  const [models, setModels] = React.useState<ModelDTO[]>([]);
  const [nodes, setNodes] = React.useState<NodeDTO[]>([]);
  const [keyword, setKeyword] = React.useState('');
  const [isLoading, setIsLoading] = React.useState(false);

  // Quick dispatch state
  const [dispatchNodeId, setDispatchNodeId] = React.useState<string>('');
  const [dispatchModelName, setDispatchModelName] = React.useState<string>('');
  const [isDispatching, setIsDispatching] = React.useState(false);

  const fetchModelsAndNodes = React.useCallback(async () => {
    try {
      setIsLoading(true);
      const [modelList, nodeList] = await Promise.all([
        AdminTranscribeService.listAllModels(keyword.trim() || undefined),
        AdminTranscribeService.listNodes(),
      ]);
      setModels(modelList);
      setNodes(nodeList.filter((n) => n.is_online));
      if (nodeList.filter((n) => n.is_online).length > 0 && !dispatchNodeId) {
        setDispatchNodeId(String(nodeList.filter((n) => n.is_online)[0].id));
      }
      if (modelList.length > 0 && !dispatchModelName) {
        setDispatchModelName(modelList[0].name);
      }
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || 'Failed to fetch models');
    } finally {
      setIsLoading(false);
    }
  }, [keyword, dispatchNodeId, dispatchModelName]);

  React.useEffect(() => {
    fetchModelsAndNodes();
  }, [fetchModelsAndNodes]);

  const handleToggleStatus = async (model: ModelDTO, nextState: boolean) => {
    try {
      await AdminTranscribeService.toggleModelStatus(model.id, nextState);
      setModels((prev) =>
        prev.map((m) =>
          m.id === model.id ? { ...m, is_active: nextState } : m,
        ),
      );
      toast.success(t('toggleSuccess'));
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || 'Failed to update model status');
    }
  };

  const handleQuickDispatch = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!dispatchNodeId || !dispatchModelName) return;

    try {
      setIsDispatching(true);
      await AdminTranscribeService.loadModel(
        Number(dispatchNodeId),
        dispatchModelName,
      );
      toast.success(t('dispatchSuccess'));
      fetchModelsAndNodes();
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || 'Dispatch failed');
    } finally {
      setIsDispatching(false);
    }
  };

  return (
    <div className='space-y-6'>
      {/* Quick Dispatch Card */}
      <Card className='border-dashed shadow-none bg-muted/20'>
        <CardHeader className='pb-3'>
          <CardTitle className='flex items-center gap-1.5 text-sm font-semibold'>
            <Send className='size-4 text-primary' />
            <span>{t('dispatchTitle')}</span>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={handleQuickDispatch}
            className='flex flex-col gap-3 sm:flex-row sm:items-end'
          >
            <div className='flex-1 space-y-1.5'>
              <span className='text-xs font-medium text-muted-foreground'>
                {t('selectNode')}
              </span>
              <Select
                value={dispatchNodeId}
                onValueChange={setDispatchNodeId}
                disabled={nodes.length === 0}
              >
                <SelectTrigger
                  className='h-8 border-dashed shadow-none text-xs bg-background'
                  aria-label={t('selectNode')}
                >
                  <SelectValue
                    placeholder={
                      nodes.length === 0
                        ? 'No online nodes available'
                        : t('selectNode')
                    }
                  />
                </SelectTrigger>
                <SelectContent>
                  {nodes.map((n) => (
                    <SelectItem key={n.id} value={String(n.id)}>
                      {n.name} (#{n.id})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className='flex-1 space-y-1.5'>
              <span className='text-xs font-medium text-muted-foreground'>
                {t('selectModel')}
              </span>
              <Select
                value={dispatchModelName}
                onValueChange={setDispatchModelName}
              >
                <SelectTrigger
                  className='h-8 border-dashed shadow-none text-xs bg-background'
                  aria-label={t('selectModel')}
                >
                  <SelectValue placeholder={t('selectModel')} />
                </SelectTrigger>
                <SelectContent>
                  {models.map((m) => (
                    <SelectItem key={m.id} value={m.name}>
                      {m.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <Button
              type='submit'
              size='sm'
              disabled={isDispatching || !dispatchNodeId || !dispatchModelName}
              className='h-8 text-xs shadow-none gap-1.5'
            >
              <Cpu className='size-3.5' />
              <span>{isDispatching ? 'Dispatching...' : t('dispatchBtn')}</span>
            </Button>
          </form>
        </CardContent>
      </Card>

      {/* Models Table */}
      <div className='space-y-4'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
          <p className='text-sm font-semibold tracking-tight text-foreground'>
            {t('title')}
          </p>
          <div className='flex items-center gap-2'>
            <div className='relative w-48'>
              <Search className='absolute left-2.5 top-2.5 size-3 text-muted-foreground' />
              <Input
                type='search'
                value={keyword}
                onChange={(e) => setKeyword(e.target.value)}
                placeholder='Search model...'
                className='h-8 pl-8 text-xs w-full shadow-none border-dashed bg-background'
              />
            </div>
            <Button
              variant='outline'
              size='sm'
              className='h-8 border-dashed text-xs shadow-none px-2.5 gap-1'
              onClick={fetchModelsAndNodes}
              disabled={isLoading}
              aria-label='Refresh'
            >
              <RefreshCw
                className={`size-3 ${isLoading ? 'animate-spin' : ''}`}
              />
              <span>Refresh</span>
            </Button>
          </div>
        </div>

        <div className='border border-dashed shadow-none rounded-lg overflow-hidden bg-background'>
          <Table className='w-full caption-bottom text-sm min-w-full'>
            <TableHeader className='bg-muted/40'>
              <TableRow className='border-dashed hover:bg-transparent'>
                <TableHead className='w-48 text-xs font-semibold'>
                  {t('modelName')}
                </TableHead>
                <TableHead className='w-36 text-xs font-semibold'>
                  {t('taskType')}
                </TableHead>
                <TableHead className='text-xs font-semibold'>
                  {t('description')}
                </TableHead>
                <TableHead className='w-28 text-xs font-semibold text-center'>
                  {t('status')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {models.length === 0 ? (
                <TableRow className='border-dashed hover:bg-transparent'>
                  <TableCell
                    colSpan={4}
                    className='h-32 text-center text-xs text-muted-foreground'
                  >
                    {isLoading ? tCommon('loading') : 'No models configured.'}
                  </TableCell>
                </TableRow>
              ) : (
                models.map((m) => (
                  <TableRow
                    key={m.id}
                    className='border-dashed hover:bg-muted/10 transition-colors'
                  >
                    <TableCell className='font-mono text-xs font-semibold'>
                      <div className='flex items-center gap-2'>
                        <Sparkles className='size-3 text-primary shrink-0' />
                        <span>{m.name}</span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <code className='rounded bg-muted px-1.5 py-0.5 text-xs font-mono'>
                        {m.task_type}
                      </code>
                    </TableCell>
                    <TableCell className='text-xs text-muted-foreground'>
                      {m.description || '-'}
                    </TableCell>
                    <TableCell className='text-center'>
                      <div className='flex items-center justify-center gap-2'>
                        <Switch
                          checked={m.is_active}
                          onCheckedChange={(val) => handleToggleStatus(m, val)}
                          aria-label={`Toggle active for ${m.name}`}
                        />
                        <span className='text-xs font-medium w-12 text-left'>
                          {m.is_active ? (
                            <span className='text-emerald-600 dark:text-emerald-400'>
                              {t('statusActive')}
                            </span>
                          ) : (
                            <span className='text-muted-foreground'>
                              {t('statusInactive')}
                            </span>
                          )}
                        </span>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </div>
    </div>
  );
}
