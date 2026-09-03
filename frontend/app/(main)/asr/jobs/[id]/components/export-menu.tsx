// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import type { JobDTO, VerboseJSONResult } from '@/lib/services/transcribe';
import { Check, Copy, Download } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

interface ExportMenuProps {
  job: JobDTO;
}

export function ExportMenu({ job }: ExportMenuProps) {
  const t = useTranslations('asr.jobDetail');
  const [copied, setCopied] = React.useState(false);

  const parsedVerbose: VerboseJSONResult | null = React.useMemo(() => {
    if (!job.result_json) return null;
    try {
      return JSON.parse(job.result_json) as VerboseJSONResult;
    } catch {
      return null;
    }
  }, [job.result_json]);

  const handleCopy = async () => {
    const textToCopy = job.result_text || parsedVerbose?.text || '';
    if (!textToCopy) return;

    await navigator.clipboard.writeText(textToCopy);
    setCopied(true);
    toast.success(t('copied'));
    setTimeout(() => setCopied(false), 2000);
  };

  const downloadFile = (
    content: string,
    filename: string,
    mimeType: string,
  ) => {
    const blob = new Blob([content], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  const formatSRTTime = (seconds: number) => {
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);
    const ms = Math.floor((seconds % 1) * 1000);
    return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')},${String(ms).padStart(3, '0')}`;
  };

  const formatVTTTime = (seconds: number) => {
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);
    const ms = Math.floor((seconds % 1) * 1000);
    return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}.${String(ms).padStart(3, '0')}`;
  };

  const exportTXT = () => {
    const text = job.result_text || parsedVerbose?.text || '';
    downloadFile(
      text,
      `${job.original_file_name || `job-${job.id}`}.txt`,
      'text/plain;charset=utf-8',
    );
  };

  const exportSRT = () => {
    if (!parsedVerbose?.segments || parsedVerbose.segments.length === 0) {
      exportTXT();
      return;
    }
    const lines: string[] = [];
    parsedVerbose.segments.forEach((seg, index) => {
      lines.push(`${index + 1}`);
      lines.push(`${formatSRTTime(seg.start)} --> ${formatSRTTime(seg.end)}`);
      lines.push(seg.text.trim());
      lines.push('');
    });
    downloadFile(
      lines.join('\n'),
      `${job.original_file_name || `job-${job.id}`}.srt`,
      'text/plain;charset=utf-8',
    );
  };

  const exportVTT = () => {
    if (!parsedVerbose?.segments || parsedVerbose.segments.length === 0) {
      exportTXT();
      return;
    }
    const lines: string[] = ['WEBVTT', ''];
    parsedVerbose.segments.forEach((seg, index) => {
      lines.push(`${index + 1}`);
      lines.push(`${formatVTTTime(seg.start)} --> ${formatVTTTime(seg.end)}`);
      lines.push(seg.text.trim());
      lines.push('');
    });
    downloadFile(
      lines.join('\n'),
      `${job.original_file_name || `job-${job.id}`}.vtt`,
      'text/vtt;charset=utf-8',
    );
  };

  const exportJSON = () => {
    const jsonStr =
      job.result_json || JSON.stringify({ text: job.result_text }, null, 2);
    downloadFile(
      jsonStr,
      `${job.original_file_name || `job-${job.id}`}.json`,
      'application/json;charset=utf-8',
    );
  };

  const hasResult = Boolean(job.result_text || job.result_json);

  return (
    <div className='flex items-center gap-2'>
      <Button
        variant='outline'
        size='sm'
        disabled={!hasResult}
        onClick={handleCopy}
        className='gap-1.5'
      >
        {copied ? (
          <Check className='size-3.5 text-emerald-500' />
        ) : (
          <Copy className='size-3.5' />
        )}
        <span>{t('copyText')}</span>
      </Button>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant='outline'
            size='sm'
            disabled={!hasResult}
            className='gap-1.5'
          >
            <Download className='size-3.5' />
            <span>{t('export')}</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end' className='w-44'>
          <DropdownMenuItem onClick={exportTXT}>
            {t('exportTxt')}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={exportSRT}>
            {t('exportSrt')}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={exportVTT}>
            {t('exportVtt')}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={exportJSON}>
            {t('exportJson')}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
