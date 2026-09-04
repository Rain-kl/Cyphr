// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { useParams, useRouter } from 'next/navigation';
import { RequireAuth } from '@/components/auth/require-auth';
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
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Progress } from '@/components/ui/progress';
import { Switch } from '@/components/ui/switch';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  AdminTranscribeService,
  type NodeDTO,
} from '@/lib/services/transcribe';
import {
  Activity,
  ArrowLeft,
  Check,
  Copy,
  Cpu,
  Eye,
  EyeOff,
  FileCode,
  HardDrive,
  Layers,
  Plus,
  RefreshCw,
  Save,
  Server,
  Settings,
  Terminal,
  Trash2,
  Wifi,
  Zap,
} from 'lucide-react';
import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

import { LoadModelDialog } from '../../asr/components/load-model-dialog';

export function NodeDetailClient() {
  const router = useRouter();
  const params = useParams();

  const rawId = React.useMemo(() => {
    if (params?.id && params.id !== '0') {
      return String(params.id);
    }
    if (typeof window !== 'undefined') {
      const match = window.location.pathname.match(/\/admin\/nodes\/([^/?#]+)/);
      if (match && match[1] && match[1] !== '0') {
        return match[1];
      }
    }
    return params?.id ? String(params.id) : '';
  }, [params?.id]);

  const [node, setNode] = React.useState<NodeDTO | null>(null);
  const [isLoading, setIsLoading] = React.useState(true);
  const [isRefreshing, setIsRefreshing] = React.useState(false);
  const [copiedKey, setCopiedKey] = React.useState<string | null>(null);
  const [loadModelOpen, setLoadModelOpen] = React.useState(false);
  const [unloadLoadingModel, setUnloadLoadingModel] = React.useState<
    string | null
  >(null);

  // Connection config controller url
  const [controllerUrl, setControllerUrl] = React.useState('');
  const [showToken, setShowToken] = React.useState(false);

  // Translations
  const tConfig = useTranslations('adminAsr.nodeConfig');

  // Active Main Tab
  const [activeTab, setActiveTab] = React.useState<'overview' | 'config'>(
    'overview',
  );

  // Node Configuration Editable State
  const [workMode, setWorkMode] = React.useState<string>('gpu');
  const [allowAutoLoad, setAllowAutoLoad] = React.useState<boolean>(true);
  const [autoUnloadMinutes, setAutoUnloadMinutes] = React.useState<number>(0);
  const [modelVramEstimates, setModelVramEstimates] = React.useState<
    Record<string, number>
  >({});
  const [newModelName, setNewModelName] = React.useState<string>('');
  const [newModelMb, setNewModelMb] = React.useState<string>('');
  const [isSavingConfig, setIsSavingConfig] = React.useState<boolean>(false);

  // Sync state when node data loads or changes
  const initialSyncRef = React.useRef(false);
  React.useEffect(() => {
    if (node && !initialSyncRef.current) {
      setWorkMode(node.work_mode || 'gpu');
      setAllowAutoLoad(node.allow_auto_load ?? true);
      setAutoUnloadMinutes(node.auto_unload_minutes ?? 0);
      setModelVramEstimates(node.model_vram_estimates || {});
      initialSyncRef.current = true;
    }
  }, [node]);

  const handleSaveConfig = async () => {
    if (!node) return;
    try {
      setIsSavingConfig(true);
      const updated = await AdminTranscribeService.updateNodeConfig(node.id, {
        work_mode: workMode,
        allow_auto_load: allowAutoLoad,
        auto_unload_minutes: Number(autoUnloadMinutes) || 0,
        model_vram_estimates: modelVramEstimates,
      });
      setNode(updated);
      setWorkMode(updated.work_mode || 'gpu');
      setAllowAutoLoad(updated.allow_auto_load ?? true);
      setAutoUnloadMinutes(updated.auto_unload_minutes ?? 0);
      setModelVramEstimates(updated.model_vram_estimates || {});
      toast.success(tConfig('saveSuccess'));
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || tConfig('saveFailed'));
    } finally {
      setIsSavingConfig(false);
    }
  };

  const handleAddModelEstimate = () => {
    const name = newModelName.trim();
    const mb = parseInt(newModelMb.trim(), 10);
    if (!name || isNaN(mb) || mb < 0) {
      toast.error('请输入有效的模型名称和显存数值 (MB)');
      return;
    }
    setModelVramEstimates((prev) => ({
      ...prev,
      [name]: mb,
    }));
    setNewModelName('');
    setNewModelMb('');
  };

  const handleRemoveModelEstimate = (name: string) => {
    setModelVramEstimates((prev) => {
      const copy = { ...prev };
      delete copy[name];
      return copy;
    });
  };

  const handleQuickAddEstimate = (name: string, mb: number) => {
    setModelVramEstimates((prev) => ({
      ...prev,
      [name]: mb,
    }));
  };

  React.useEffect(() => {
    if (typeof window !== 'undefined') {
      setControllerUrl(window.location.origin);
    }
  }, []);

  const fetchNode = React.useCallback(
    async (isBackground = false) => {
      if (!rawId || rawId === '0') {
        setIsLoading(false);
        return;
      }
      try {
        if (!isBackground) {
          setIsRefreshing(true);
        }
        const data = await AdminTranscribeService.getNode(rawId);
        setNode(data);
      } catch (err: unknown) {
        const error = err as Error;
        toast.error(error.message || '获取节点详情失败');
      } finally {
        setIsLoading(false);
        setIsRefreshing(false);
      }
    },
    [rawId],
  );

  React.useEffect(() => {
    fetchNode();
  }, [fetchNode]);

  // Periodic polling for node metrics
  React.useEffect(() => {
    if (!rawId || rawId === '0') return;
    const timer = setInterval(() => {
      fetchNode(true);
    }, 5000);
    return () => clearInterval(timer);
  }, [rawId, fetchNode]);

  const handleCopy = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(key);
    toast.success('已复制到剪贴板');
    setTimeout(() => {
      setCopiedKey((prev) => (prev === key ? null : prev));
    }, 2000);
  };

  const handleUnloadModel = async (modelName: string) => {
    if (!node) return;
    try {
      setUnloadLoadingModel(modelName);
      await AdminTranscribeService.unloadModel(node.id, modelName);
      toast.success(`已下发卸载模型指令: ${modelName}`);
      await fetchNode(true);
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || '卸载模型失败');
    } finally {
      setUnloadLoadingModel(null);
    }
  };

  const handleDeleteNode = async () => {
    if (!node) return;
    try {
      await AdminTranscribeService.deleteNode(node.id);
      toast.success('节点删除成功');
      router.push('/admin/nodes');
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || '删除节点失败');
    }
  };

  if (isLoading) {
    return (
      <RequireAuth>
        <div className='flex h-96 w-full items-center justify-center'>
          <RefreshCw className='size-8 animate-spin text-primary/60' />
        </div>
      </RequireAuth>
    );
  }

  if (!node) {
    return (
      <RequireAuth>
        <div className='flex h-96 w-full flex-col items-center justify-center gap-4 text-center'>
          <Server className='size-12 text-muted-foreground/40' />
          <h2 className='text-lg font-semibold'>未找到指定的节点</h2>
          <p className='text-sm text-muted-foreground'>
            节点可能已被注销或不存在 (ID: #{rawId})
          </p>
          <Button onClick={() => router.push('/admin/nodes')} variant='outline'>
            <ArrowLeft className='mr-2 size-4' /> 返回节点列表
          </Button>
        </div>
      </RequireAuth>
    );
  }

  const sys = node.system;
  const effectiveUrl = controllerUrl.trim() || 'http://localhost:8000';
  const actualToken = node.agent_token || '';

  // Snippets
  const shellSnippet = `CONTROLLER_URL="${effectiveUrl}" \\
AGENT_TOKEN="${actualToken}" \\
NODE_NAME="${node.name}" \\
uv run python -m src.main`;

  const yamlSnippet = `controller_url: "${effectiveUrl}"
agent_token: "${actualToken}"
node_name: "${node.name}"
heartbeat_interval: 5
max_concurrent_jobs: 2`;

  const dockerSnippet = `docker run -d \\
  --name cyphr-agent-${node.name} \\
  --gpus all \\
  --restart unless-stopped \\
  -e CONTROLLER_URL="${effectiveUrl}" \\
  -e AGENT_TOKEN="${actualToken}" \\
  -e NODE_NAME="${node.name}" \\
  cyphr-agent:latest`;

  const systemdSnippet = `[Unit]
Description=Cyphr ASR Inference Agent - ${node.name}
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/home/ubuntu/Applications/Cyphr/backend/agent
Environment="CONTROLLER_URL=${effectiveUrl}"
Environment="AGENT_TOKEN=${actualToken}"
Environment="NODE_NAME=${node.name}"
ExecStart=/home/ubuntu/.local/bin/uv run python -m src.main
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target`;

  const formatRAM = (used?: number, total?: number) => {
    if (!used && !total) return '-';
    return `${((used || 0) / 1024).toFixed(1)} / ${((total || 0) / 1024).toFixed(1)} GB`;
  };

  const formatVRAM = (used?: number, total?: number) => {
    if (!used && !total) return '-';
    return `${used || 0} / ${total || 0} MB`;
  };

  return (
    <RequireAuth>
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3 }}
        className='flex w-full flex-col gap-6 py-6 px-1 max-w-7xl mx-auto'
      >
        {/* Navigation & Header */}
        <div className='flex flex-col gap-4'>
          <div>
            <Button
              variant='ghost'
              size='sm'
              onClick={() => router.push('/admin/nodes')}
              className='gap-1.5 text-muted-foreground hover:text-foreground -ml-2 h-8 px-2'
            >
              <ArrowLeft className='size-4' />
              <span>返回节点列表</span>
            </Button>
          </div>

          <div className='flex flex-col md:flex-row md:items-center justify-between gap-4 border-b pb-5'>
            <div className='flex items-start gap-3.5'>
              <div className='rounded-xl bg-primary/10 p-3 text-primary shrink-0 mt-0.5'>
                <Server className='size-6' />
              </div>
              <div>
                <div className='flex items-center gap-2.5 flex-wrap'>
                  <h1 className='text-2xl font-bold tracking-tight'>
                    {node.name}
                  </h1>
                  {node.is_online ? (
                    <Badge
                      variant='outline'
                      className='gap-1.5 border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 py-0.5'
                    >
                      <span className='size-2 rounded-full bg-emerald-500 animate-pulse' />
                      <span>在线 (Online)</span>
                    </Badge>
                  ) : (
                    <Badge
                      variant='outline'
                      className='gap-1.5 border-muted bg-muted/30 text-muted-foreground py-0.5'
                    >
                      <span className='size-2 rounded-full bg-muted-foreground/50' />
                      <span>离线 (Offline)</span>
                    </Badge>
                  )}
                  <Badge variant='secondary' className='font-mono text-xs'>
                    #{node.id}
                  </Badge>
                  {node.current_mode && (
                    <Badge
                      variant='outline'
                      className='gap-1 border-primary/30 bg-primary/10 text-primary py-0.5 text-xs'
                    >
                      <Cpu className='size-3' />
                      <span className='uppercase font-semibold'>
                        {node.current_mode}
                      </span>
                    </Badge>
                  )}
                </div>
                <p className='text-xs text-muted-foreground mt-1'>
                  上次连接 IP:{' '}
                  <span className='font-mono text-foreground/80'>
                    {node.last_ip || '暂无 IP'}
                  </span>{' '}
                  • Token 前缀:{' '}
                  <span className='font-mono text-foreground/80'>
                    {node.token_prefix || '-'}
                  </span>
                </p>
              </div>
            </div>

            <div className='flex items-center gap-2.5'>
              <Button
                variant='outline'
                size='sm'
                onClick={() => fetchNode()}
                disabled={isRefreshing}
                className='h-9 gap-1.5'
              >
                <RefreshCw
                  className={`size-3.5 ${isRefreshing ? 'animate-spin' : ''}`}
                />
                <span>刷新</span>
              </Button>

              <Button
                size='sm'
                variant='default'
                disabled={!node.is_online}
                onClick={() => setLoadModelOpen(true)}
                className='h-9 gap-1.5 shadow-sm'
              >
                <Plus className='size-3.5' />
                <span>加载模型</span>
              </Button>

              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button
                    variant='outline'
                    size='sm'
                    className='h-9 text-rose-600 hover:text-rose-700 hover:bg-rose-500/10 border-rose-200 dark:border-rose-900/50'
                  >
                    <Trash2 className='size-3.5 mr-1' />
                    <span>删除节点</span>
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>
                      确定删除节点 &ldquo;{node.name}&rdquo;？
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                      删除后该节点的连接凭证将立即永久作废，正在运行的连接会话将被强制断开。
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>取消</AlertDialogCancel>
                    <AlertDialogAction
                      onClick={handleDeleteNode}
                      className='bg-rose-600 hover:bg-rose-700 text-white'
                    >
                      确认删除
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </div>
          </div>
        </div>

        {/* Top-level Navigation Tabs */}
        <Tabs
          value={activeTab}
          onValueChange={(val) => setActiveTab(val as 'overview' | 'config')}
          className='w-full'
        >
          <TabsList className='h-10 p-1 bg-muted/60 mb-2'>
            <TabsTrigger value='overview' className='gap-2 text-sm px-4'>
              <Activity className='size-4' />
              <span>{tConfig('tabOverview')}</span>
            </TabsTrigger>
            <TabsTrigger value='config' className='gap-2 text-sm px-4'>
              <Settings className='size-4' />
              <span>{tConfig('tabConfig')}</span>
            </TabsTrigger>
          </TabsList>

          {/* Tab 1: Overview & Telemetry */}
          <TabsContent value='overview' className='mt-4 space-y-6'>
            {/* Top 4 Summary Metrics */}
            <div className='grid grid-cols-2 gap-4 md:grid-cols-4'>
              <Card className='border shadow-sm'>
                <CardContent className='flex items-center justify-between p-4'>
                  <div>
                    <p className='text-xs font-medium text-muted-foreground'>
                      连接状态
                    </p>
                    <div className='mt-1 text-lg font-bold'>
                      {node.is_online ? (
                        <span className='text-emerald-600 dark:text-emerald-400'>
                          正常连接
                        </span>
                      ) : (
                        <span className='text-muted-foreground'>等待上线</span>
                      )}
                    </div>
                  </div>
                  <div
                    className={`rounded-xl p-2.5 ${node.is_online ? 'bg-emerald-500/10 text-emerald-500' : 'bg-muted text-muted-foreground'}`}
                  >
                    <Wifi className='size-5' />
                  </div>
                </CardContent>
              </Card>

              <Card className='border shadow-sm'>
                <CardContent className='flex items-center justify-between p-4'>
                  <div>
                    <p className='text-xs font-medium text-muted-foreground'>
                      并发运行作业
                    </p>
                    <div className='mt-1 text-2xl font-bold tracking-tight text-amber-600 dark:text-amber-400'>
                      {node.running_jobs || 0}
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
                      CPU 使用率
                    </p>
                    <div className='mt-1 text-2xl font-bold tracking-tight font-mono'>
                      {sys ? `${sys.cpu_percent.toFixed(1)}%` : '-'}
                    </div>
                  </div>
                  <div className='rounded-xl p-2.5 bg-blue-500/10 text-blue-500'>
                    <Cpu className='size-5' />
                  </div>
                </CardContent>
              </Card>

              <Card className='border shadow-sm'>
                <CardContent className='flex items-center justify-between p-4'>
                  <div>
                    <p className='text-xs font-medium text-muted-foreground'>
                      已载入模型数
                    </p>
                    <div className='mt-1 text-2xl font-bold tracking-tight text-primary'>
                      {node.loaded_models?.length || 0}
                    </div>
                  </div>
                  <div className='rounded-xl p-2.5 bg-primary/10 text-primary'>
                    <Zap className='size-5' />
                  </div>
                </CardContent>
              </Card>
            </div>

            {/* Two Column Section: Telemetry & Models */}
            <div className='grid grid-cols-1 lg:grid-cols-3 gap-6'>
              {/* Left 2 Cols: Hardware Telemetry */}
              <Card className='lg:col-span-2 border shadow-sm'>
                <CardHeader className='pb-3'>
                  <div className='flex items-center justify-between'>
                    <div className='flex items-center gap-2'>
                      <Activity className='size-4 text-primary' />
                      <CardTitle className='text-base'>
                        实时硬件遥测指标
                      </CardTitle>
                    </div>
                    {node.last_seen_at && (
                      <span className='text-xs text-muted-foreground font-mono'>
                        上次心跳:{' '}
                        {new Date(node.last_seen_at).toLocaleTimeString()}
                      </span>
                    )}
                  </div>
                  <CardDescription>
                    Agent 客户端上报的处理器、系统内存与显存使用情况
                  </CardDescription>
                </CardHeader>
                <CardContent className='space-y-6 pt-2'>
                  {/* CPU Metric */}
                  <div className='space-y-2'>
                    <div className='flex items-center justify-between text-sm'>
                      <span className='flex items-center gap-1.5 font-medium'>
                        <Cpu className='size-4 text-blue-500' />
                        CPU 处理器负载
                      </span>
                      <span className='font-mono font-semibold'>
                        {sys ? `${sys.cpu_percent.toFixed(1)}%` : '未上报'}
                      </span>
                    </div>
                    <Progress
                      value={sys?.cpu_percent || 0}
                      className='h-2.5 bg-muted'
                    />
                  </div>

                  {/* RAM Metric */}
                  <div className='space-y-2'>
                    <div className='flex items-center justify-between text-sm'>
                      <span className='flex items-center gap-1.5 font-medium'>
                        <HardDrive className='size-4 text-emerald-500' />
                        系统物理内存 (RAM)
                      </span>
                      <span className='font-mono font-semibold'>
                        {sys && sys.ram_percent !== undefined
                          ? `${sys.ram_percent.toFixed(1)}% (${formatRAM(sys.ram_used_mb, sys.ram_total_mb)})`
                          : '未上报'}
                      </span>
                    </div>
                    <Progress
                      value={sys?.ram_percent || 0}
                      className='h-2.5 bg-muted'
                    />
                  </div>

                  {/* GPU VRAM Metric */}
                  <div className='space-y-2'>
                    <div className='flex items-center justify-between text-sm'>
                      <span className='flex items-center gap-1.5 font-medium'>
                        <Zap className='size-4 text-purple-500' />
                        GPU 显存占用 (VRAM)
                      </span>
                      <span className='font-mono font-semibold'>
                        {sys?.gpu_memory_total_mb &&
                        sys.gpu_memory_used_mb !== undefined &&
                        sys.gpu_memory_total_mb > 0
                          ? `${((sys.gpu_memory_used_mb / sys.gpu_memory_total_mb) * 100).toFixed(1)}% (${formatVRAM(sys.gpu_memory_used_mb, sys.gpu_memory_total_mb)})`
                          : sys?.gpu_percent !== undefined
                            ? `${sys.gpu_percent.toFixed(1)}%`
                            : 'CPU 节点或无可用独立显卡'}
                      </span>
                    </div>
                    <Progress
                      value={
                        sys?.gpu_memory_total_mb &&
                        sys.gpu_memory_used_mb !== undefined &&
                        sys.gpu_memory_total_mb > 0
                          ? (sys.gpu_memory_used_mb / sys.gpu_memory_total_mb) *
                            100
                          : sys?.gpu_percent || 0
                      }
                      className='h-2.5 bg-muted'
                    />
                  </div>

                  {/* Info grid */}
                  <div className='grid grid-cols-2 sm:grid-cols-4 gap-3 pt-3 border-t text-xs'>
                    <div>
                      <span className='text-muted-foreground block'>
                        创建时间
                      </span>
                      <span className='font-mono font-medium mt-0.5 block'>
                        {new Date(node.created_at).toLocaleDateString()}
                      </span>
                    </div>
                    <div>
                      <span className='text-muted-foreground block'>
                        心跳周期
                      </span>
                      <span className='font-mono font-medium mt-0.5 block'>
                        5 秒
                      </span>
                    </div>
                    <div>
                      <span className='text-muted-foreground block'>
                        网络 IP
                      </span>
                      <span className='font-mono font-medium mt-0.5 block truncate'>
                        {node.last_ip || '未记录'}
                      </span>
                    </div>
                    <div>
                      <span className='text-muted-foreground block'>
                        调度状态
                      </span>
                      <span className='font-medium mt-0.5 block text-emerald-600 dark:text-emerald-400'>
                        就绪调度
                      </span>
                    </div>
                  </div>
                </CardContent>
              </Card>

              {/* Right 1 Col: Loaded Models */}
              <Card className='border shadow-sm flex flex-col justify-between'>
                <CardHeader className='pb-3'>
                  <div className='flex items-center justify-between'>
                    <div className='flex items-center gap-2'>
                      <Zap className='size-4 text-primary' />
                      <CardTitle className='text-base'>已载入模型</CardTitle>
                    </div>
                    <Badge variant='secondary' className='text-xs'>
                      {node.loaded_models?.length || 0} 个模型
                    </Badge>
                  </div>
                  <CardDescription>
                    当前驻留在该节点显存/内存中的 ASR 模型
                  </CardDescription>
                </CardHeader>
                <CardContent className='flex-1 flex flex-col justify-between'>
                  {node.loaded_models && node.loaded_models.length > 0 ? (
                    <div className='space-y-2.5'>
                      {node.loaded_models.map((modelName) => (
                        <div
                          key={modelName}
                          className='flex items-center justify-between rounded-lg border bg-muted/20 p-2.5 text-xs'
                        >
                          <div className='flex items-center gap-2 overflow-hidden'>
                            <span className='size-2 rounded-full bg-primary shrink-0' />
                            <span className='font-mono font-semibold truncate'>
                              {modelName}
                            </span>
                          </div>
                          <Button
                            size='sm'
                            variant='ghost'
                            disabled={unloadLoadingModel === modelName}
                            onClick={() => handleUnloadModel(modelName)}
                            className='h-7 text-xs text-rose-600 hover:text-rose-700 hover:bg-rose-500/10 px-2'
                          >
                            {unloadLoadingModel === modelName ? (
                              <RefreshCw className='size-3 animate-spin' />
                            ) : (
                              '卸载'
                            )}
                          </Button>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className='flex flex-col items-center justify-center rounded-lg border border-dashed py-8 text-center text-muted-foreground my-auto'>
                      <Zap className='size-8 opacity-40 mb-2' />
                      <p className='text-xs font-medium'>当前节点未载入模型</p>
                      <p className='text-[11px] text-muted-foreground/70 mt-0.5'>
                        系统将在作业调度时自动分发，或手动点击加载
                      </p>
                    </div>
                  )}

                  <Button
                    variant='outline'
                    size='sm'
                    disabled={!node.is_online}
                    onClick={() => setLoadModelOpen(true)}
                    className='w-full mt-4 h-8 text-xs gap-1.5'
                  >
                    <Plus className='size-3.5' />
                    <span>加载新模型</span>
                  </Button>
                </CardContent>
              </Card>
            </div>

            {/* Section 3: Connection Configuration (一键复制连接配置信息) */}
            <Card className='border shadow-sm'>
              <CardHeader className='pb-3'>
                <div className='flex items-center justify-between flex-wrap gap-2'>
                  <div className='flex items-center gap-2'>
                    <Terminal className='size-5 text-primary' />
                    <div>
                      <CardTitle className='text-base'>
                        节点连接配置信息
                      </CardTitle>
                      <CardDescription className='mt-0.5'>
                        复制启动参数或配置文件，在远程计算主机或 GPU
                        服务器上快速启动 Agent 接入该节点
                      </CardDescription>
                    </div>
                  </div>
                </div>
              </CardHeader>

              <CardContent className='space-y-5'>
                {/* Controller URL & Token Settings */}
                <div className='rounded-xl border bg-muted/20 p-4 space-y-4'>
                  <div className='flex flex-col sm:flex-row sm:items-center justify-between gap-3'>
                    <div className='space-y-0.5'>
                      <Label
                        htmlFor='controller-url'
                        className='text-xs font-semibold'
                      >
                        Controller 服务端地址 (CONTROLLER_URL)
                      </Label>
                      <p className='text-xs text-muted-foreground'>
                        Agent 节点连接 Controller WebSocket
                        与上报日志所用的基础地址
                      </p>
                    </div>
                    <div className='w-full sm:w-80'>
                      <Input
                        id='controller-url'
                        value={controllerUrl}
                        onChange={(e) => setControllerUrl(e.target.value)}
                        placeholder='http://your-server-ip:8000'
                        className='font-mono text-xs h-8 bg-background'
                      />
                    </div>
                  </div>

                  <div className='flex flex-col sm:flex-row sm:items-center justify-between gap-3 pt-3 border-t'>
                    <div className='space-y-0.5'>
                      <Label
                        htmlFor='agent-token-input'
                        className='text-xs font-semibold'
                      >
                        节点通信凭证 (AGENT_TOKEN)
                      </Label>
                      <p className='text-xs text-muted-foreground'>
                        用于 Agent 客户端与 Controller 建立鉴权与 WebSocket
                        双向握手（已持久保存）
                      </p>
                    </div>
                    <div className='flex items-center gap-1.5 w-full sm:w-80'>
                      <div className='relative flex-1'>
                        <Input
                          id='agent-token-input'
                          type={showToken ? 'text' : 'password'}
                          value={actualToken}
                          readOnly
                          className='font-mono text-xs h-8 bg-background pr-8 select-all'
                        />
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon'
                          onClick={() => setShowToken((v) => !v)}
                          className='absolute right-0 top-0 h-8 w-8 text-muted-foreground hover:text-foreground'
                          aria-label={showToken ? '隐藏 Token' : '显示 Token'}
                        >
                          {showToken ? (
                            <EyeOff className='size-3.5' />
                          ) : (
                            <Eye className='size-3.5' />
                          )}
                        </Button>
                      </div>
                      <Button
                        type='button'
                        variant='outline'
                        size='icon'
                        onClick={() => handleCopy(actualToken, 'token')}
                        className='h-8 w-8 shrink-0'
                        aria-label='复制 Token'
                      >
                        {copiedKey === 'token' ? (
                          <Check className='size-3.5 text-emerald-500' />
                        ) : (
                          <Copy className='size-3.5' />
                        )}
                      </Button>
                    </div>
                  </div>
                </div>

                {/* Config Tabs */}
                <Tabs defaultValue='shell' className='w-full'>
                  <div className='flex items-center justify-between flex-wrap gap-2 mb-2'>
                    <TabsList className='h-9'>
                      <TabsTrigger value='shell' className='text-xs gap-1.5'>
                        <Terminal className='size-3.5' />
                        <span>Shell 环境变量</span>
                      </TabsTrigger>
                      <TabsTrigger value='yaml' className='text-xs gap-1.5'>
                        <FileCode className='size-3.5' />
                        <span>config.yaml 配置文件</span>
                      </TabsTrigger>
                      <TabsTrigger value='docker' className='text-xs gap-1.5'>
                        <Server className='size-3.5' />
                        <span>Docker 容器启动</span>
                      </TabsTrigger>
                      <TabsTrigger value='systemd' className='text-xs gap-1.5'>
                        <Zap className='size-3.5' />
                        <span>Systemd 服务配置</span>
                      </TabsTrigger>
                    </TabsList>
                  </div>

                  {/* Shell Tab */}
                  <TabsContent value='shell' className='mt-2 space-y-2'>
                    <div className='relative rounded-xl border bg-zinc-950 p-4 text-zinc-100 font-mono text-xs overflow-x-auto'>
                      <Button
                        size='sm'
                        variant='secondary'
                        onClick={() => handleCopy(shellSnippet, 'shell')}
                        className='absolute top-3 right-3 h-7 gap-1.5 text-xs bg-zinc-800 hover:bg-zinc-700 text-zinc-100 border-zinc-700'
                      >
                        {copiedKey === 'shell' ? (
                          <Check className='size-3.5 text-emerald-400' />
                        ) : (
                          <Copy className='size-3.5' />
                        )}
                        <span>
                          {copiedKey === 'shell' ? '已复制' : '一键复制'}
                        </span>
                      </Button>
                      <pre className='pr-24 leading-relaxed'>
                        {shellSnippet}
                      </pre>
                    </div>
                    <p className='text-[11px] text-muted-foreground'>
                      说明：进入{' '}
                      <code className='font-mono'>backend/agent</code>{' '}
                      目录，执行上方指令即可直接以前台模式启动 Agent 进程。
                    </p>
                  </TabsContent>

                  {/* YAML Tab */}
                  <TabsContent value='yaml' className='mt-2 space-y-2'>
                    <div className='relative rounded-xl border bg-zinc-950 p-4 text-zinc-100 font-mono text-xs overflow-x-auto'>
                      <Button
                        size='sm'
                        variant='secondary'
                        onClick={() => handleCopy(yamlSnippet, 'yaml')}
                        className='absolute top-3 right-3 h-7 gap-1.5 text-xs bg-zinc-800 hover:bg-zinc-700 text-zinc-100 border-zinc-700'
                      >
                        {copiedKey === 'yaml' ? (
                          <Check className='size-3.5 text-emerald-400' />
                        ) : (
                          <Copy className='size-3.5' />
                        )}
                        <span>
                          {copiedKey === 'yaml' ? '已复制' : '一键复制'}
                        </span>
                      </Button>
                      <pre className='pr-24 leading-relaxed'>{yamlSnippet}</pre>
                    </div>
                    <p className='text-[11px] text-muted-foreground'>
                      说明：保存至{' '}
                      <code className='font-mono'>
                        backend/agent/config.yaml
                      </code>
                      ，随后执行{' '}
                      <code className='font-mono'>
                        uv run python -m src.main
                      </code>
                      。
                    </p>
                  </TabsContent>

                  {/* Docker Tab */}
                  <TabsContent value='docker' className='mt-2 space-y-2'>
                    <div className='relative rounded-xl border bg-zinc-950 p-4 text-zinc-100 font-mono text-xs overflow-x-auto'>
                      <Button
                        size='sm'
                        variant='secondary'
                        onClick={() => handleCopy(dockerSnippet, 'docker')}
                        className='absolute top-3 right-3 h-7 gap-1.5 text-xs bg-zinc-800 hover:bg-zinc-700 text-zinc-100 border-zinc-700'
                      >
                        {copiedKey === 'docker' ? (
                          <Check className='size-3.5 text-emerald-400' />
                        ) : (
                          <Copy className='size-3.5' />
                        )}
                        <span>
                          {copiedKey === 'docker' ? '已复制' : '一键复制'}
                        </span>
                      </Button>
                      <pre className='pr-24 leading-relaxed'>
                        {dockerSnippet}
                      </pre>
                    </div>
                    <p className='text-[11px] text-muted-foreground'>
                      说明：适用于容器化 GPU 部署（需宿主机安装 NVIDIA Container
                      Toolkit）。
                    </p>
                  </TabsContent>

                  {/* Systemd Tab */}
                  <TabsContent value='systemd' className='mt-2 space-y-2'>
                    <div className='relative rounded-xl border bg-zinc-950 p-4 text-zinc-100 font-mono text-xs overflow-x-auto'>
                      <Button
                        size='sm'
                        variant='secondary'
                        onClick={() => handleCopy(systemdSnippet, 'systemd')}
                        className='absolute top-3 right-3 h-7 gap-1.5 text-xs bg-zinc-800 hover:bg-zinc-700 text-zinc-100 border-zinc-700'
                      >
                        {copiedKey === 'systemd' ? (
                          <Check className='size-3.5 text-emerald-400' />
                        ) : (
                          <Copy className='size-3.5' />
                        )}
                        <span>
                          {copiedKey === 'systemd' ? '已复制' : '一键复制'}
                        </span>
                      </Button>
                      <pre className='pr-24 leading-relaxed'>
                        {systemdSnippet}
                      </pre>
                    </div>
                    <p className='text-[11px] text-muted-foreground'>
                      说明：保存至{' '}
                      <code className='font-mono'>
                        /etc/systemd/system/cyphr-agent.service
                      </code>
                      ，执行{' '}
                      <code className='font-mono'>
                        systemctl enable --now cyphr-agent
                      </code>{' '}
                      设置开机自启。
                    </p>
                  </TabsContent>
                </Tabs>
              </CardContent>
            </Card>
          </TabsContent>

          {/* Tab 2: Node Configuration & VRAM */}
          <TabsContent value='config' className='mt-4 space-y-6'>
            {/* Section: Inference Work Mode */}
            <Card className='border shadow-sm'>
              <CardHeader className='pb-3'>
                <div className='flex items-center justify-between'>
                  <div className='flex items-center gap-2'>
                    <Cpu className='size-4 text-primary' />
                    <CardTitle className='text-base'>
                      {tConfig('workModeTitle')}
                    </CardTitle>
                  </div>
                  {node.current_mode && (
                    <Badge
                      variant='outline'
                      className='gap-1 border-primary/30 bg-primary/10 text-primary text-xs'
                    >
                      <span>
                        {tConfig('currentModeBadge')}:{' '}
                        {node.current_mode.toUpperCase()}
                      </span>
                    </Badge>
                  )}
                </div>
                <CardDescription>{tConfig('workModeDesc')}</CardDescription>
              </CardHeader>
              <CardContent>
                <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
                  {/* GPU Mode Card */}
                  {(() => {
                    const gpuSupported =
                      !node.supported_modes ||
                      node.supported_modes.length === 0 ||
                      node.supported_modes.includes('gpu');
                    const isSelected = workMode === 'gpu';
                    return (
                      <div
                        onClick={() => {
                          if (gpuSupported) setWorkMode('gpu');
                        }}
                        className={`relative flex flex-col justify-between rounded-xl border p-4 transition-all ${
                          !gpuSupported
                            ? 'opacity-50 cursor-not-allowed border-dashed bg-muted/20'
                            : isSelected
                              ? 'border-primary bg-primary/5 cursor-pointer ring-1 ring-primary'
                              : 'border-border hover:border-primary/50 cursor-pointer bg-card'
                        }`}
                      >
                        <div className='flex items-start justify-between gap-3'>
                          <div className='flex items-center gap-2.5'>
                            <div
                              className={`rounded-lg p-2 ${isSelected ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'}`}
                            >
                              <Zap className='size-4' />
                            </div>
                            <div>
                              <p className='font-semibold text-sm'>
                                {tConfig('modeGpu')}
                              </p>
                              <p className='text-xs text-muted-foreground mt-0.5'>
                                {gpuSupported
                                  ? '启用显卡张量加速核心，适合高速批量生产环境'
                                  : tConfig('gpuDisabledTip')}
                              </p>
                            </div>
                          </div>
                          {isSelected && (
                            <span className='size-2 rounded-full bg-primary shrink-0 mt-1' />
                          )}
                        </div>
                      </div>
                    );
                  })()}

                  {/* CPU Mode Card */}
                  {(() => {
                    const isSelected = workMode === 'cpu';
                    return (
                      <div
                        onClick={() => setWorkMode('cpu')}
                        className={`relative flex flex-col justify-between rounded-xl border p-4 transition-all cursor-pointer ${
                          isSelected
                            ? 'border-primary bg-primary/5 ring-1 ring-primary'
                            : 'border-border hover:border-primary/50 bg-card'
                        }`}
                      >
                        <div className='flex items-start justify-between gap-3'>
                          <div className='flex items-center gap-2.5'>
                            <div
                              className={`rounded-lg p-2 ${isSelected ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'}`}
                            >
                              <Cpu className='size-4' />
                            </div>
                            <div>
                              <p className='font-semibold text-sm'>
                                {tConfig('modeCpu')}
                              </p>
                              <p className='text-xs text-muted-foreground mt-0.5'>
                                纯 CPU 主机通用模式，不占用或要求显卡显存
                              </p>
                            </div>
                          </div>
                          {isSelected && (
                            <span className='size-2 rounded-full bg-primary shrink-0 mt-1' />
                          )}
                        </div>
                      </div>
                    );
                  })()}
                </div>
              </CardContent>
            </Card>

            {/* Section: Auto-Load & Idle Auto-Unload */}
            <Card className='border shadow-sm'>
              <CardHeader className='pb-3'>
                <div className='flex items-center gap-2'>
                  <Activity className='size-4 text-primary' />
                  <CardTitle className='text-base'>
                    调度与生命周期策略
                  </CardTitle>
                </div>
                <CardDescription>
                  配置自动加载模型以及长时间空闲时自动卸载模型以释放显存
                </CardDescription>
              </CardHeader>
              <CardContent className='space-y-6 pt-2'>
                {/* Allow Auto Load Switch */}
                <div className='flex flex-col sm:flex-row sm:items-center justify-between gap-4 rounded-xl border bg-muted/20 p-4'>
                  <div className='space-y-0.5'>
                    <Label
                      htmlFor='auto-load-switch'
                      className='text-sm font-semibold cursor-pointer'
                    >
                      {tConfig('autoLoadTitle')}
                    </Label>
                    <p className='text-xs text-muted-foreground'>
                      {tConfig('autoLoadDesc')}
                    </p>
                  </div>
                  <Switch
                    id='auto-load-switch'
                    checked={allowAutoLoad}
                    onCheckedChange={setAllowAutoLoad}
                  />
                </div>

                {/* Auto Unload Minutes Input */}
                <div className='flex flex-col sm:flex-row sm:items-center justify-between gap-4 rounded-xl border bg-muted/20 p-4'>
                  <div className='space-y-0.5'>
                    <Label
                      htmlFor='auto-unload-minutes'
                      className='text-sm font-semibold'
                    >
                      {tConfig('autoUnloadTitle')}
                    </Label>
                    <p className='text-xs text-muted-foreground'>
                      {tConfig('autoUnloadDesc')}
                    </p>
                  </div>
                  <div className='flex items-center gap-2 w-full sm:w-44'>
                    <Input
                      id='auto-unload-minutes'
                      type='number'
                      min={0}
                      max={1440}
                      value={autoUnloadMinutes}
                      onChange={(e) =>
                        setAutoUnloadMinutes(parseInt(e.target.value, 10) || 0)
                      }
                      className='h-9 font-mono text-sm bg-background'
                    />
                    <span className='text-xs text-muted-foreground shrink-0'>
                      分钟
                    </span>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Section: Model VRAM Estimates Table */}
            <Card className='border shadow-sm'>
              <CardHeader className='pb-3'>
                <div className='flex items-center justify-between'>
                  <div className='flex items-center gap-2'>
                    <Zap className='size-4 text-primary' />
                    <CardTitle className='text-base'>
                      {tConfig('vramTitle')}
                    </CardTitle>
                  </div>
                  <Badge variant='secondary' className='text-xs'>
                    {Object.keys(modelVramEstimates).length} 个配置项
                  </Badge>
                </div>
                <CardDescription>{tConfig('vramDesc')}</CardDescription>
              </CardHeader>
              <CardContent className='space-y-4 pt-1'>
                {/* Quick Add Pills */}
                <div className='flex items-center gap-2 flex-wrap'>
                  <span className='text-xs text-muted-foreground'>
                    {tConfig('quickAdd')}:
                  </span>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() =>
                      handleQuickAddEstimate('qwen3-asr-0.6b', 2000)
                    }
                    className='h-7 text-xs gap-1 font-mono'
                  >
                    <Plus className='size-3' />
                    <span>qwen3-asr-0.6b (2000 MB)</span>
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() =>
                      handleQuickAddEstimate('qwen3-asr-1.7b', 5000)
                    }
                    className='h-7 text-xs gap-1 font-mono'
                  >
                    <Plus className='size-3' />
                    <span>qwen3-asr-1.7b (5000 MB)</span>
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() =>
                      handleQuickAddEstimate('mock-whisper-base', 500)
                    }
                    className='h-7 text-xs gap-1 font-mono'
                  >
                    <Plus className='size-3' />
                    <span>mock-whisper-base (500 MB)</span>
                  </Button>
                </div>

                {/* Estimates List Table */}
                <div className='rounded-xl border overflow-hidden'>
                  <table className='w-full text-xs'>
                    <thead>
                      <tr className='border-b bg-muted/40 text-muted-foreground font-medium'>
                        <th className='text-left py-2.5 px-3'>
                          {tConfig('modelName')}
                        </th>
                        <th className='text-left py-2.5 px-3'>
                          {tConfig('estimateMb')}
                        </th>
                        <th className='text-right py-2.5 px-3'>操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {Object.keys(modelVramEstimates).length === 0 ? (
                        <tr>
                          <td
                            colSpan={3}
                            className='py-6 text-center text-muted-foreground'
                          >
                            {tConfig('noEstimates')}
                          </td>
                        </tr>
                      ) : (
                        Object.entries(modelVramEstimates).map(
                          ([mName, mb]) => (
                            <tr
                              key={mName}
                              className='border-b last:border-b-0 hover:bg-muted/10'
                            >
                              <td className='py-2.5 px-3 font-mono font-medium'>
                                {mName}
                              </td>
                              <td className='py-2.5 px-3 font-mono text-purple-600 dark:text-purple-400 font-semibold'>
                                {mb} MB
                              </td>
                              <td className='py-2.5 px-3 text-right'>
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='sm'
                                  onClick={() =>
                                    handleRemoveModelEstimate(mName)
                                  }
                                  className='h-7 px-2 text-rose-600 hover:text-rose-700 hover:bg-rose-500/10'
                                >
                                  <Trash2 className='size-3.5' />
                                </Button>
                              </td>
                            </tr>
                          ),
                        )
                      )}
                    </tbody>
                  </table>
                </div>

                {/* Add Custom Estimate Form */}
                <div className='flex flex-col sm:flex-row items-center gap-2 pt-1'>
                  <Input
                    value={newModelName}
                    onChange={(e) => setNewModelName(e.target.value)}
                    placeholder={tConfig('modelPlaceholder')}
                    className='h-9 text-xs font-mono'
                  />
                  <Input
                    type='number'
                    min={0}
                    value={newModelMb}
                    onChange={(e) => setNewModelMb(e.target.value)}
                    placeholder={tConfig('mbPlaceholder')}
                    className='h-9 text-xs font-mono sm:w-40'
                  />
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={handleAddModelEstimate}
                    className='h-9 text-xs gap-1.5 shrink-0 w-full sm:w-auto'
                  >
                    <Plus className='size-3.5' />
                    <span>{tConfig('addEstimate')}</span>
                  </Button>
                </div>
              </CardContent>
            </Card>

            {/* Action Footer: Save Button */}
            <div className='flex justify-end pt-2 pb-6'>
              <Button
                onClick={handleSaveConfig}
                disabled={isSavingConfig}
                className='gap-2 min-w-36 h-10 shadow-sm'
              >
                {isSavingConfig ? (
                  <RefreshCw className='size-4 animate-spin' />
                ) : (
                  <Save className='size-4' />
                )}
                <span>{tConfig('saveConfig')}</span>
              </Button>
            </div>
          </TabsContent>
        </Tabs>

        {/* Load Model Dialog */}
        <LoadModelDialog
          node={node}
          open={loadModelOpen}
          onOpenChange={setLoadModelOpen}
          onSuccess={() => fetchNode(true)}
        />
      </motion.div>
    </RequireAuth>
  );
}
