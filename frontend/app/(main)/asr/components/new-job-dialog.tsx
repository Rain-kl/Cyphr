// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

'use client';

import * as React from 'react';
import { useRouter } from 'next/navigation';
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
import { Slider } from '@/components/ui/slider';
import { Textarea } from '@/components/ui/textarea';
import { TranscribeService, type ModelDTO } from '@/lib/services/transcribe';
import {
  ChevronDown,
  ChevronUp,
  FileAudio,
  FileVideo,
  Sparkles,
  UploadCloud,
  X,
} from 'lucide-react';
import { useTranslations } from 'next-intl';
import { toast } from 'sonner';

interface NewJobDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess?: () => void;
}

export function NewJobDialog({
  open,
  onOpenChange,
  onSuccess,
}: NewJobDialogProps) {
  const router = useRouter();
  const t = useTranslations('asr.dialog');

  const [file, setFile] = React.useState<File | null>(null);
  const [models, setModels] = React.useState<ModelDTO[]>([]);
  const [selectedModel, setSelectedModel] = React.useState<string>('');
  const [taskType, setTaskType] = React.useState<string>('transcribe');
  const [language, setLanguage] = React.useState<string>('auto');
  const [prompt, setPrompt] = React.useState<string>('');
  const [temperature, setTemperature] = React.useState<number>(0);
  const [showAdvanced, setShowAdvanced] = React.useState<boolean>(false);
  const [isSubmitting, setIsSubmitting] = React.useState<boolean>(false);
  const [isDragging, setIsDragging] = React.useState<boolean>(false);

  const fileInputRef = React.useRef<HTMLInputElement | null>(null);

  React.useEffect(() => {
    if (open) {
      TranscribeService.listModels()
        .then((data) => {
          setModels(data);
          if (data.length > 0 && !selectedModel) {
            setSelectedModel(data[0].name);
          }
        })
        .catch((err) => {
          console.error('Failed to load models:', err);
        });
    }
  }, [open, selectedModel]);

  const handleFileChange = (selected: File | null) => {
    if (!selected) return;
    setFile(selected);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      handleFileChange(e.dataTransfer.files[0]);
    }
  };

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024 * 1024) {
      return `${(bytes / 1024).toFixed(1)} KB`;
    }
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!file) {
      toast.error(t('fileRequired'));
      return;
    }

    try {
      setIsSubmitting(true);
      const formData = new FormData();
      formData.append('file', file);
      formData.append('model', selectedModel || 'mock-whisper-base');
      formData.append('task_type', taskType);
      if (language && language !== 'auto') {
        formData.append('language', language);
      }
      if (prompt.trim()) {
        formData.append('prompt', prompt.trim());
      }
      if (temperature > 0) {
        formData.append('temperature', String(temperature));
      }

      const res = await TranscribeService.submitTranscription(formData);
      toast.success(t('submitSuccess'));
      onOpenChange(false);
      if (onSuccess) onSuccess();
      router.push(`/asr/jobs/${res.job_id}`);
    } catch (err: unknown) {
      const error = err as Error;
      toast.error(error.message || 'Submit failed');
    } finally {
      setIsSubmitting(false);
    }
  };

  const resetForm = () => {
    setFile(null);
    setPrompt('');
    setTemperature(0);
    setShowAdvanced(false);
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) resetForm();
        onOpenChange(v);
      }}
    >
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2 text-xl font-semibold'>
            <Sparkles className='size-5 text-primary' />
            {t('title')}
          </DialogTitle>
          <DialogDescription>{t('description')}</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className='space-y-4 pt-1'>
          {/* File Drag and Drop Zone */}
          <input
            ref={fileInputRef}
            type='file'
            accept='audio/*,video/*,.mp3,.wav,.m4a,.flac,.aac,.ogg,.mp4,.mkv,.mov,.flv,.webm'
            className='hidden'
            onChange={(e) => {
              if (e.target.files && e.target.files.length > 0) {
                handleFileChange(e.target.files[0]);
              }
            }}
          />

          {!file ? (
            <div
              onDragOver={(e) => {
                e.preventDefault();
                setIsDragging(true);
              }}
              onDragLeave={() => setIsDragging(false)}
              onDrop={handleDrop}
              onClick={() => fileInputRef.current?.click()}
              className={`flex cursor-pointer flex-col items-center justify-center rounded-xl border-2 border-dashed p-6 text-center transition-colors ${
                isDragging
                  ? 'border-primary bg-primary/5'
                  : 'border-muted-foreground/25 hover:border-primary/50 hover:bg-muted/30'
              }`}
            >
              <div className='rounded-full bg-primary/10 p-3 text-primary mb-2'>
                <UploadCloud className='size-6' />
              </div>
              <p className='text-sm font-medium'>{t('dropzoneText')}</p>
              <p className='mt-1 text-xs text-muted-foreground'>
                {t('supportedFormats')}
              </p>
            </div>
          ) : (
            <div className='flex items-center justify-between rounded-xl border bg-muted/30 p-3'>
              <div className='flex items-center gap-3 overflow-hidden'>
                <div className='rounded-lg bg-primary/10 p-2 text-primary shrink-0'>
                  {file.type.startsWith('video/') ? (
                    <FileVideo className='size-5' />
                  ) : (
                    <FileAudio className='size-5' />
                  )}
                </div>
                <div className='min-w-0'>
                  <p className='truncate text-sm font-medium'>{file.name}</p>
                  <p className='text-xs text-muted-foreground'>
                    {formatFileSize(file.size)}
                  </p>
                </div>
              </div>
              <Button
                type='button'
                variant='ghost'
                size='icon'
                className='size-8 text-muted-foreground hover:text-foreground shrink-0'
                onClick={() => setFile(null)}
                aria-label={t('removeFile')}
              >
                <X className='size-4' />
              </Button>
            </div>
          )}

          {/* Model Selection */}
          <div className='grid grid-cols-2 gap-3'>
            <div className='space-y-1.5'>
              <Label htmlFor='model-select' className='text-xs font-medium'>
                {t('modelLabel')}
              </Label>
              <Select value={selectedModel} onValueChange={setSelectedModel}>
                <SelectTrigger id='model-select' aria-label={t('modelLabel')}>
                  <SelectValue placeholder='Select model' />
                </SelectTrigger>
                <SelectContent>
                  {models.map((m) => (
                    <SelectItem key={m.id} value={m.name}>
                      {m.name}
                    </SelectItem>
                  ))}
                  {models.length === 0 && (
                    <SelectItem value='mock-whisper-base'>
                      mock-whisper-base
                    </SelectItem>
                  )}
                </SelectContent>
              </Select>
            </div>

            {/* Task Type Selection */}
            <div className='space-y-1.5'>
              <Label htmlFor='task-type' className='text-xs font-medium'>
                {t('taskTypeLabel')}
              </Label>
              <Select value={taskType} onValueChange={setTaskType}>
                <SelectTrigger id='task-type' aria-label={t('taskTypeLabel')}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='transcribe'>
                    {t('typeTranscribe')}
                  </SelectItem>
                  <SelectItem value='translate'>
                    {t('typeTranslate')}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          {/* Language Selection */}
          <div className='space-y-1.5'>
            <Label htmlFor='language-select' className='text-xs font-medium'>
              {t('languageLabel')}
            </Label>
            <Select value={language} onValueChange={setLanguage}>
              <SelectTrigger
                id='language-select'
                aria-label={t('languageLabel')}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='auto'>{t('langAuto')}</SelectItem>
                <SelectItem value='zh'>{t('langZh')}</SelectItem>
                <SelectItem value='en'>{t('langEn')}</SelectItem>
                <SelectItem value='ja'>{t('langJa')}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {/* Collapsible Advanced Options */}
          <div className='border-t pt-2'>
            <button
              type='button'
              onClick={() => setShowAdvanced(!showAdvanced)}
              className='flex w-full items-center justify-between py-1 text-xs font-medium text-muted-foreground hover:text-foreground'
            >
              <span>{t('advancedOptions')}</span>
              {showAdvanced ? (
                <ChevronUp className='size-3.5' />
              ) : (
                <ChevronDown className='size-3.5' />
              )}
            </button>

            {showAdvanced && (
              <div className='space-y-3 pt-2'>
                <div className='space-y-1.5'>
                  <Label htmlFor='prompt-input' className='text-xs font-medium'>
                    {t('promptLabel')}
                  </Label>
                  <Textarea
                    id='prompt-input'
                    rows={2}
                    value={prompt}
                    onChange={(e) => setPrompt(e.target.value)}
                    placeholder={t('promptPlaceholder')}
                    className='text-xs'
                  />
                </div>

                <div className='space-y-1.5'>
                  <div className='flex items-center justify-between'>
                    <Label
                      htmlFor='temp-slider'
                      className='text-xs font-medium'
                    >
                      {t('temperatureLabel')}
                    </Label>
                    <span className='text-xs font-mono text-muted-foreground'>
                      {temperature.toFixed(2)}
                    </span>
                  </div>
                  <Slider
                    id='temp-slider'
                    value={[temperature]}
                    min={0}
                    max={1}
                    step={0.05}
                    onValueChange={(val) => setTemperature(val[0])}
                  />
                </div>
              </div>
            )}
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
            <Button type='submit' disabled={isSubmitting || !file}>
              {isSubmitting ? t('submitting') : t('submit')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
