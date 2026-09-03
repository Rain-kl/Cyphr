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
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  AdminTranscribeService,
  type NodeCreatedDTO,
} from '@/lib/services/transcribe';
import { Check, Copy, Key, Server, Terminal } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

interface CreateNodeDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess: () => void;
}

export function CreateNodeDialog({
  open,
  onOpenChange,
  onSuccess,
}: CreateNodeDialogProps) {
  const t = useTranslations('adminAsr.createNodeDialog');

  const [name, setName] = React.useState('');
  const [isSubmitting, setIsSubmitting] = React.useState(false);
  const [createdNode, setCreatedNode] = React.useState<NodeCreatedDTO | null>(
    null,
  );
  const [copiedToken, setCopiedToken] = React.useState(false);
  const [copiedCmd, setCopiedCmd] = React.useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;

    try {
      setIsSubmitting(true);
      const res = await AdminTranscribeService.createNode(name.trim());
      setCreatedNode(res);
      onSuccess();
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || 'Failed to create node');
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleCopy = async (text: string, type: 'token' | 'cmd') => {
    await navigator.clipboard.writeText(text);
    if (type === 'token') {
      setCopiedToken(true);
      setTimeout(() => setCopiedToken(false), 2000);
    } else {
      setCopiedCmd(true);
      setTimeout(() => setCopiedCmd(false), 2000);
    }
    toast.success(t('copied'));
  };

  const reset = () => {
    setName('');
    setCreatedNode(null);
    setCopiedToken(false);
    setCopiedCmd(false);
  };

  const controllerUrl =
    typeof window !== 'undefined'
      ? `${window.location.protocol}//${window.location.host}`
      : 'http://localhost:8080';

  const launchCommand = createdNode
    ? `CONTROLLER_URL=${controllerUrl} AGENT_TOKEN=${createdNode.agent_token} NODE_NAME=${createdNode.name} uv run python -m src.main`
    : '';

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) reset();
        onOpenChange(v);
      }}
    >
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2 text-xl font-semibold'>
            <Server className='size-5 text-primary' />
            {createdNode ? t('successTitle') : t('title')}
          </DialogTitle>
          <DialogDescription>{t('desc')}</DialogDescription>
        </DialogHeader>

        {!createdNode ? (
          <form onSubmit={handleSubmit} className='space-y-4 pt-2'>
            <div className='space-y-2'>
              <Label htmlFor='node-name-input' className='text-xs font-medium'>
                {t('nameLabel')}
              </Label>
              <Input
                id='node-name-input'
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t('namePlaceholder')}
                required
                autoFocus
              />
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
              <Button type='submit' disabled={isSubmitting || !name.trim()}>
                {isSubmitting ? 'Creating...' : t('create')}
              </Button>
            </DialogFooter>
          </form>
        ) : (
          <div className='space-y-4 pt-2'>
            {/* Token Block */}
            <div className='space-y-1.5'>
              <div className='flex items-center gap-1.5 text-xs font-medium text-amber-600 dark:text-amber-400'>
                <Key className='size-3.5' />
                <span>{t('tokenNotice')}</span>
              </div>
              <div className='flex items-center gap-2 rounded-lg border bg-muted/40 p-2.5 font-mono text-xs'>
                <span className='flex-1 truncate select-all'>
                  {createdNode.agent_token}
                </span>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='size-7 shrink-0'
                  onClick={() => handleCopy(createdNode.agent_token, 'token')}
                  aria-label={t('copyToken')}
                >
                  {copiedToken ? (
                    <Check className='size-3.5 text-emerald-500' />
                  ) : (
                    <Copy className='size-3.5' />
                  )}
                </Button>
              </div>
            </div>

            {/* Launch Command Block */}
            <div className='space-y-1.5'>
              <div className='flex items-center gap-1.5 text-xs font-medium text-muted-foreground'>
                <Terminal className='size-3.5 text-primary' />
                <span>{t('launchCommand')}</span>
              </div>
              <div className='flex items-center gap-2 rounded-lg border bg-zinc-950 p-2.5 font-mono text-xs text-zinc-100 dark:bg-zinc-900'>
                <span className='flex-1 overflow-x-auto select-all whitespace-pre-wrap break-all text-[11px] leading-relaxed'>
                  {launchCommand}
                </span>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='size-7 shrink-0 text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800'
                  onClick={() => handleCopy(launchCommand, 'cmd')}
                  aria-label={t('copyCommand')}
                >
                  {copiedCmd ? (
                    <Check className='size-3.5 text-emerald-500' />
                  ) : (
                    <Copy className='size-3.5' />
                  )}
                </Button>
              </div>
            </div>

            <DialogFooter className='pt-2'>
              <Button
                type='button'
                onClick={() => onOpenChange(false)}
                className='w-full sm:w-auto'
              >
                {t('close')}
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
